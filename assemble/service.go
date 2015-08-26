package assemble

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"time"

	"github.com/Jeffail/gabs"
	"github.com/googollee/go-socket.io"
	"github.com/satori/go.uuid"

	"github.com/blamarche/assemble-web-chat/config"
	"github.com/blamarche/assemble-web-chat/utils"
)

// Service holds the key parts of the assemble server
type Service struct {
	SocketServer *socketio.Server

	Cfg         *config.Config
	Rooms       map[string]*Room
	Users       map[string]*User
	OnlineUsers map[string]*OnlineUser
	Invites     map[string]string
	Banlist     map[string]string
	UserKey     []byte
	DefMaxExp   time.Duration
	DefMinExp   time.Duration
}

// NewService creates an assemble server
func NewService(cfg *config.Config, userkey []byte) *Service {
	s := Service{}
	s.Cfg = cfg

	s.Rooms = make(map[string]*Room, 100)
	s.Users = make(map[string]*User, 100)
	s.OnlineUsers = make(map[string]*OnlineUser, 100)
	s.Banlist = make(map[string]string, 100)
	s.Invites = make(map[string]string, 100)

	s.UserKey = userkey

	//setup initial lobby
	max, err := time.ParseDuration(cfg.DefaultMaxExp)
	if err != nil {
		log.Fatal("Invalid max exp: " + cfg.DefaultMaxExp)
	}
	s.DefMaxExp = max
	min, err := time.ParseDuration(cfg.DefaultMinExp)
	if err != nil {
		log.Fatal("Invalid min exp: " + cfg.DefaultMinExp)
	}
	s.DefMinExp = min

	s.CreateRoom("Lobby", "lobby", false, "", s.DefMaxExp, s.DefMinExp, "", 100)

	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	s.SocketServer = server

	return &s
}

// Start listens and sets up goroutines
func (svc *Service) Start() {
	//setup history expiration goroutine
	go svc.expireHistory()

	//setup timeout for online status
	go svc.onlineUserTimeout()

	//setup alert job
	go svc.alertSender()

	//ListenAndServe
	log.Println("Serving at " + svc.Cfg.Host + svc.Cfg.Bind)
	if svc.Cfg.Bind == ":80" {
		http.ListenAndServe(svc.Cfg.Bind, nil)
	} else {
		http.ListenAndServeTLS(svc.Cfg.Bind, "cert.pem", "key.pem", nil)
	}
}

func (svc *Service) alertSender() {
	wait, err := time.ParseDuration(svc.Cfg.LastAlertWait)
	_ = wait
	if err != nil {
		log.Fatal("LastAlertWait invalid value")
	}

	d, _ := time.ParseDuration("5s")
	for {
		for k, v := range svc.Rooms {
			if k != "lobby" && len(v.Messages) > 0 {
				lastmsgstamp := v.Messages[len(v.Messages)-1].Path("time").Data().(int64)

				//TODO client-side option with auth that disables notifications per-user
				for uid := range v.MemberUIDs {
					_, isonline := svc.OnlineUsers[uid]
					if !isonline {
						alertaddr := svc.Users[uid].Token.Path("alertaddress").Data()
						if alertaddr != nil && lastmsgstamp > svc.Users[uid].LastAlert.Unix() && lastmsgstamp > svc.Users[uid].LastAct.Unix() {
							diff := time.Now().Sub(svc.Users[uid].LastAlert)
							if diff > wait {
								svc.Users[uid].LastAlert = time.Now()
								svc.SendAlert(alertaddr.(string), "Assemble", "New messages in "+v.FriendlyName)
							}
						}
					}
				}
			}
		}
		time.Sleep(d)
	}
}

func (svc *Service) onlineUserTimeout() {
	d, _ := time.ParseDuration(svc.Cfg.UserTimeout) //5 minute timeout
	for {
		for k, v := range svc.OnlineUsers {
			diff := time.Now().Sub(v.LastPing)
			if diff > d {
				delete(svc.OnlineUsers, k)

				rms := (*v.So).Rooms()
				for i := 0; i < len(rms); i++ {
					svc.BroadcastUserLeave(rms[i], k, *v.So)
				}

				(*v.So).BroadcastTo("lobby", "userdisconnect", `{"uid":"`+k+`"}`)
			}
		}

		time.Sleep(d)
	}
}

func (svc *Service) expireHistory() {
	server := svc.SocketServer
	d, _ := time.ParseDuration("30s")
	for {
		//log.Println("Running history expiration")
		for k, v := range svc.Rooms {
			c := 0
			for i := len(v.Messages) - 1; i >= 0; i-- {
				mtt := v.Messages[i].Path("time").Data().(int64)
				mt := time.Unix(mtt, 0)
				diff := time.Now().Sub(mt)
				durstr := v.Messages[i].Path("dur").Data().(string)
				dur, err := time.ParseDuration(durstr)

				//log.Println(mt, time.Now(), diff, durstr, err)
				if err != nil || diff >= dur {
					server.BroadcastTo(k, "deletechatm", v.Messages[i].Path("msgid").Data().(string))
					v.Messages = append(v.Messages[:i], v.Messages[i+1:]...)
					c++
				}
			}
			if c > 0 {
				log.Println("Expired", c, "messages from", k)
			}

			c = 0
			if len(v.Messages) > v.MaxHistoryLen {
				for {
					if len(v.Messages) <= v.MaxHistoryLen {
						break
					}

					server.BroadcastTo(k, "deletechatm", v.Messages[0].Path("msgid").Data().(string))
					v.Messages = append(v.Messages[:0], v.Messages[1:]...)
					c++
				}
			}
			if c > 0 {
				log.Println("Trimmed", c, "messages from", k)
			}
		}

		time.Sleep(d)
	}
}

// CreateRoom adds a room to the services list
func (svc *Service) CreateRoom(fname, roomid string, isprivate bool, creatoruid string, maxexptime time.Duration, minexptime time.Duration, avatar string, maxhistorylen int) *Room {
	r := Room{}
	r.FriendlyName = fname
	r.RoomID = roomid
	r.IsPrivate = isprivate
	r.CreatorUID = creatoruid
	r.MaxExpTime = maxexptime
	r.MinExpTime = minexptime
	r.Avatar = avatar
	r.MaxHistoryLen = maxhistorylen

	r.Messages = make([]*gabs.Container, 0)
	r.MemberUIDs = make(map[string]string, 10)
	r.InvitedUIDs = make(map[string]string, 10)
	/*
		if r.CreatorUID != "" {
			r.MemberUIDs[r.CreatorUID] = r.CreatorUID //ineffecient, perhaps a pointer to the actual gabs object
		}
	*/
	svc.Rooms[roomid] = &r

	return &r
}

//TODO STILL NEEDS REFACTORING

func (svc *Service) SendAlert(toaddr, subject, message string) {
	if svc.Cfg.SMTP.Enabled {
		//log.Println("alert", toaddr, subject, message)
		from := mail.Address{"", svc.Cfg.SMTP.From}
		to := mail.Address{"", toaddr}
		subj := subject
		body := message

		// Setup headers
		headers := make(map[string]string)
		headers["From"] = from.String()
		headers["To"] = to.String()
		headers["Subject"] = subj

		// Setup message
		message := ""
		for k, v := range headers {
			message += fmt.Sprintf("%s: %s\r\n", k, v)
		}
		message += "\r\n" + body

		// Connect to the SMTP Server
		servername := svc.Cfg.SMTP.SslHostPort
		host, _, _ := net.SplitHostPort(servername)
		auth := smtp.PlainAuth("", svc.Cfg.SMTP.Username, svc.Cfg.SMTP.Password, host)

		// TLS config
		tlsconfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         host,
		}

		//hardcoded to support TLS only (ie. port 465 smtp servers)
		conn, err := tls.Dial("tcp", servername, tlsconfig)
		if err != nil {
			log.Println("smtperr", err)
		}
		c, err := smtp.NewClient(conn, host)
		if err != nil {
			log.Println("smtperr", err)
		}

		// Auth
		if err = c.Auth(auth); err != nil {
			log.Println("smtperr", err)
		}
		// To && From
		if err = c.Mail(from.Address); err != nil {
			log.Println("smtperr", err)
		}
		if err = c.Rcpt(to.Address); err != nil {
			log.Println("smtperr", err)
		}
		// Data
		w, err := c.Data()
		if err != nil {
			log.Println("smtperr", err)
		}
		_, err = w.Write([]byte(message))
		if err != nil {
			log.Println("smtperr", err)
		}
		err = w.Close()
		if err != nil {
			log.Println("smtperr", err)
		}
		c.Quit()
	}
}

func (svc *Service) SendOnlineUserList(so socketio.Socket) {
	cu, _ := gabs.ParseJSON([]byte("{}"))
	uids := []string{}
	nicks := []string{}
	for k := range svc.OnlineUsers {
		uids = append(uids, k)
		nicks = append(nicks, svc.Users[k].Token.Path("nick").Data().(string))
	}
	cu.SetP(uids, "uids")
	cu.SetP(nicks, "nicks")

	so.Emit("onlineusers", cu.String())
}

func (svc *Service) BroadcastUserLeave(room string, uid string, so socketio.Socket) {
	bc, _ := gabs.ParseJSON([]byte("{}"))
	bc.SetP(svc.Users[uid].Token.Path("uid").Data().(string), "uid")
	bc.SetP(room, "room")
	so.BroadcastTo(room, "leave", bc.String())
	so.Emit("leave", bc.String())
}

func (svc *Service) CreateRoomList() string {
	list, _ := gabs.ParseJSON([]byte("{}"))

	for k, v := range svc.Rooms {
		if !v.IsPrivate {
			list.SetP(v.FriendlyName, k)
		}
	}

	return list.String()
}

func (svc *Service) CanJoin(uid string, room string, removeinvite bool) bool {
	if !svc.Rooms[room].IsPrivate {
		return true
	}

	_, ok := svc.Rooms[room].InvitedUIDs[uid]
	if ok {
		delete(svc.Rooms[room].InvitedUIDs, uid)
		return true
	}
	if svc.Rooms[room].CreatorUID == uid {
		return true
	}
	return false
}

func (svc *Service) JoinRooms(so socketio.Socket, uid string) {
	for k, v := range svc.Rooms {
		_, ok := v.MemberUIDs[uid]
		if ok {
			svc.JoinRoom(so, uid, k)
			//log.Println(h.String())
		}
	}
}

func (svc *Service) AddToRoom(so socketio.Socket, uid string, room string) {
	svc.Rooms[room].MemberUIDs[uid] = uid
	if so != nil {
		svc.JoinRoom(so, uid, room)
	}
}

func (svc *Service) JoinRoom(so socketio.Socket, uid string, room string) {
	v := svc.Rooms[room]
	k := room

	//check for permission
	_, ok := v.MemberUIDs[uid]
	if !ok {
		return
	}

	so.Join(k)

	jo, _ := gabs.ParseJSON([]byte("{}"))
	jo.SetP(v.MaxExpTime.String(), "maxexptime")
	jo.SetP(v.MinExpTime.String(), "minexptime")
	jo.SetP(k, "room")
	jo.SetP(v.FriendlyName, "name")

	so.Emit("join", jo.String())

	bc, _ := gabs.ParseJSON([]byte("{}"))
	bc.SetP(svc.Users[uid].Token.Path("nick").Data().(string), "nick")
	bc.SetP(uid, "uid")
	bc.SetP(k, "room")
	bc.SetP(v.FriendlyName, "name")
	so.BroadcastTo(k, "joined", bc.String())

	svc.SendRoomHistory(so, uid, room)
}

func (svc *Service) SendRoomHistory(so socketio.Socket, uid string, room string) {
	v := svc.Rooms[room]
	k := room

	//send chat history
	history := "{\"history\":["
	for j := 0; j < len(v.Messages); j++ {
		history += v.Messages[j].String()
		if j < len(v.Messages)-1 {
			history += ","
		}
	}
	history += "]}"

	h, _ := gabs.ParseJSON([]byte("{}"))
	m, _ := gabs.ParseJSON([]byte(history))

	h.SetP(m.Path("history").Data(), "history")
	h.SetP(k, "room")
	h.SetP(v.FriendlyName, "name")
	so.Emit("history", h.String())
}

func (svc *Service) ExtractAndCheckToken(so socketio.Socket, g *gabs.Container) (string, bool) {
	if g.Path("t").Data() == nil {
		so.Emit("auth_error", "Missing Token")
		return "", false
	}

	uid, err := svc.ValidateUserToken(nil, g.Path("t").Data().(string))
	if err != nil {
		so.Emit("auth_error", "Invalid Token")
		return "", false
	}

	return uid, true
}

func PublicUserString(token *gabs.Container) string {
	s, err := gabs.ParseJSON([]byte(token.String()))
	if err != nil {
		return "{}"
	}

	s.SetP("", "privid")
	s.SetP("", "alertaddress")

	return s.String()
}

func (svc *Service) ValidateUserToken(so socketio.Socket, msg string) (string, error) {
	s, err := base64.StdEncoding.DecodeString(msg)
	if err != nil {
		return "", err
	}
	byt := []byte(s)

	tok, err := utils.Decrypt(svc.UserKey, byt)
	if err != nil {
		return "", err
	}

	uncomp, err := utils.Uncompress(tok)
	if err != nil {
		return "", err
	}

	token, err := gabs.ParseJSON(uncomp.Bytes())
	if err != nil {
		return "", err
	}

	uid := token.Path("uid").Data().(string)
	bandate, isbanned := svc.Banlist[uid]
	if isbanned {
		return "", &Err{"Banned on " + bandate}
	}

	_, ok := svc.Users[uid]
	if !ok {
		svc.Users[uid] = &User{}
		svc.Users[uid].LastAlert = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
		svc.Users[uid].LastAct = time.Now()
		svc.Users[uid].Token = token
		if so != nil {
			svc.AddToRoom(so, uid, "lobby")
		} else {
			svc.Rooms["lobby"].MemberUIDs[uid] = uid
		}
	} else {
		if svc.Users[uid].Token.Path("privid").Data().(string) != token.Path("privid").Data().(string) {
			return "", &Err{"Bad privID"}
		}
	}

	return uid, nil
}

func CreateNewUserToken(nick string, name string, email string, phone string, url string, desc string, avatar string, alertaddress string) (*gabs.Container, error) {

	uid := uuid.NewV4().String()
	privid := uuid.NewV4().String()
	token, err := gabs.ParseJSON([]byte(`{
		"uid":null, "privid":null, "nick":null,
		"name":null, "email":null, "phone":null,
		"url":null, "desc":null, "avatar":null
		}`))

	token.SetP(uid, "uid")
	token.SetP(privid, "privid")

	token.SetP(nick, "nick")
	token.SetP(name, "name")
	token.SetP(email, "email")
	token.SetP(phone, "phone")
	token.SetP(url, "url")
	token.SetP(desc, "desc")
	token.SetP(avatar, "avatar")
	token.SetP(alertaddress, "alertaddress")

	return token, err
}
