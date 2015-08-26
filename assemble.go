/*
This file is part of Assemble Web Chat.

Foobar is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Foobar is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with Assemble Web Chat.  If not, see <http://www.gnu.org/licenses/>.
*/
// This is not meant to be clean from the get-go and I'm sure it'll need refactoring
package main

//TODO investigate first-refresh join of lobby, etc

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Jeffail/gabs"
	"github.com/googollee/go-socket.io"
	"github.com/kabukky/httpscerts"
	"github.com/satori/go.uuid"
)

// Room contains all room info, messages, user list, etc
type Room struct {
	FriendlyName  string
	RoomID        string
	IsPrivate     bool
	CreatorUID    string
	MemberUIDs    map[string]string
	InvitedUIDs   map[string]string
	MaxExpTime    time.Duration
	MinExpTime    time.Duration
	Avatar        string
	MaxHistoryLen int
	Messages      []*gabs.Container
}

// OnlineUser has info for and online user including last active timestamp
type OnlineUser struct {
	So      *socketio.Socket
	LastAct time.Time
}

// Err general error holder
type Err struct {
	m string
}

func (e *Err) Error() string {
	return e.m
}

//Globals, eww
var rooms map[string]*Room
var users map[string]*gabs.Container
var onlineusers map[string]*OnlineUser
var invites map[string]string
var banlist map[string]string
var userkey []byte
var adminpass = "PASS"
var host = "localhost"
var port = ":443"
var defaultinvite = ""

//MAIN FUNC
func main() {
	fc, _ := ioutil.ReadFile("./userkey.txt")
	if fc != nil {
		userkey = fc[:]
	} else {
		tmp := uuid.NewV4().String()
		tmp = tmp[4:]
		ioutil.WriteFile("./userkey.txt", []byte(tmp), 0666)
		userkey = []byte(tmp)
	}

	rooms = make(map[string]*Room, 100)
	users = make(map[string]*gabs.Container, 100)
	onlineusers = make(map[string]*OnlineUser, 100)
	banlist = make(map[string]string, 100)
	invites = make(map[string]string, 100)

	//setup initial lobby
	dur1h, _ := time.ParseDuration("1h")
	dur30s, _ := time.ParseDuration("30s")
	rooms["lobby"] = createRoom("Lobby", "lobby", false, "", dur1h, dur30s, "", 100)

	//read params from terminal
	if len(os.Args) > 1 {
		host = os.Args[1]
		if host == "--help" {
			fmt.Println("usage: assemble <host> <port> <adminpass> <invitekey>")
			return
		}
		if len(os.Args) > 2 {
			port = os.Args[2]
		}
		if len(os.Args) > 3 {
			adminpass = os.Args[3]
		}
		if len(os.Args) > 4 {
			defaultinvite = os.Args[4]
		}
	}

	if defaultinvite != "" {
		invites[defaultinvite] = "admin@localhost"
	}

	// Check if the cert files are available and make new ones if needed
	err := httpscerts.Check("cert.pem", "key.pem")
	if err != nil {
		err = httpscerts.Generate("cert.pem", "key.pem", host)
		if err != nil {
			log.Fatal("Error: Couldn't create https certs.")
		}
	}

	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}

	//TODO Harden / error-proof this stuff
	//TODO an error shouldnt halt the whole thing
	server.On("connection", socketHandlers)

	server.On("error", func(so socketio.Socket, err error) {
		log.Println("error:", err)
	})

	http.Handle("/socket.io/", server)
	http.HandleFunc("/signup/", signup)
	http.HandleFunc("/login/", login)
	http.Handle("/", http.FileServer(http.Dir("./static")))

	//setup history expiration goroutine
	go expireHistory(server)

	//setup timeout for online status
	go onlineUserTimeout(server)

	log.Println("Serving at " + host + port)
	//log.Fatal(http.ListenAndServeTLS(port, "cert.pem", "key.pem", nil))
	if port == ":80" {
		http.ListenAndServe(port, nil)
	} else {
		http.ListenAndServeTLS(port, "cert.pem", "key.pem", nil)
	}
}

func socketHandlers(so socketio.Socket) {
	so.On("auth", func(msg string) {
		uid, err := validateUserToken(nil, msg)
		if err != nil {
			so.Emit("auth_error", "Invalid Token")
			if msg != "" {
				log.Println("Invalid or banned user attempt")
			}
			return
		}

		log.Println("auth", uid)
		so.Emit("auth", `success`)

		//send list of online users
		sendOnlineUserList(so)

		//add user to online status
		ou := OnlineUser{&so, time.Now()}
		onlineusers[uid] = &ou
		//since the user end sup joining lobby anyway, no need to broadcast any global online status alert

		so.On("disconnection", func() {
			//send disconnect message to all rooms they are in
			for k, v := range onlineusers {
				if v.So == &so {
					delete(onlineusers, k)

					rms := so.Rooms()
					for i := 0; i < len(rms); i++ {
						broadcastUserLeave(rms[i], k, so)
					}

					so.BroadcastTo("lobby", "userdisconnect", `{"uid":"`+k+`"}`)
					break
				}
			}
		})

		joinRooms(so, uid)
	})

	so.On("inviteusertoroom", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := extractAndCheckToken(so, g)
		if !ok {
			return
		}

		invuid := g.Path("uid").Data().(string)
		_, uok := users[invuid]
		if uok {
			invuser, invok := onlineusers[invuid]
			if !invok {
				so.Emit("auth_error", "Offline uid, try again later")
				return
			}

			room := g.Path("room").Data().(string)
			r, rok := rooms[room]
			if rok {
				if r.IsPrivate {
					caninvite := false
					if r.CreatorUID == uid {
						caninvite = true
					} else {
						_, ismember := r.MemberUIDs[uid]
						if !ismember {
							caninvite = false
						}
					}

					if !caninvite {
						so.Emit("auth_error", "You aren't a member of that room!")
						return
					}

					//finally passed the tests... lets add to the invite list of uids
					r.InvitedUIDs[invuid] = time.Now().String()
				}

				name := rooms[room].FriendlyName
				(*invuser.So).Emit("inviteusertoroom", `{"room":"`+room+`", "name":"`+name+`"}`)
			}
		} else {
			so.Emit("auth_error", "Invalid uid")
		}
	})

	so.On("userinfo", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		_, ok := extractAndCheckToken(so, g)
		if !ok {
			return
		}

		uid := g.Path("uid").Data().(string)
		u, uok := users[uid]
		if uok {
			so.Emit("userinfo", publicUserString(u))
		} else {
			so.Emit("auth_error", "Invalid uid")
		}
	})

	so.On("onlineusers", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		_, ok := extractAndCheckToken(so, g)
		if !ok {
			return
		}

		sendOnlineUserList(so)
	})

	so.On("ping", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := extractAndCheckToken(so, g)
		if !ok {
			return
		}

		//TODO optimize for network performance
		_, ok2 := onlineusers[uid]
		if ok2 {
			onlineusers[uid].LastAct = time.Now()
		}
	})

	so.On("invitenewuser", func(msg string) {
		//log.Println(msg)
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := extractAndCheckToken(so, g)
		if !ok {
			return
		}

		em := g.Path("email").Data().(string)
		fmt.Println(uid, "invited", em)

		//TODO email invite
		id := uuid.NewV4().String()
		invites[id] = em

		so.Emit("invitenewuser", "{\"key\": \""+id+"\"}")
	})

	so.On("roomlist", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		_, ok := extractAndCheckToken(so, g)
		if !ok {
			return
		}

		so.Emit("roomlist", createRoomList())
	})

	so.On("ban", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		_, ok := extractAndCheckToken(so, g)
		if !ok {
			return
		}

		//TODO save/load bans to disk
		pass := g.Path("pass").Data().(string)
		banid := g.Path("uid").Data().(string)

		if pass == adminpass {
			banlist[banid] = time.Now().String()
			so.Emit("ban", "Banned")
		} else {
			so.Emit("auth_error", "Bad admin password")
		}
	})

	so.On("unban", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		_, ok := extractAndCheckToken(so, g)
		if !ok {
			return
		}

		//TODO save/load bans to disk
		pass := g.Path("pass").Data().(string)
		banid := g.Path("uid").Data().(string)

		if pass == adminpass {
			delete(banlist, banid)
			so.Emit("ban", "Unbanned")
		} else {
			so.Emit("auth_error", "Bad admin password")
			return
		}
	})

	so.On("directmessage", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := extractAndCheckToken(so, g)
		if !ok {
			return
		}

		muid := g.Path("uid").Data().(string)

		//check users
		mu, isonline := onlineusers[muid]
		if !isonline {
			so.Emit("auth_error", "User must be online to initiate direct messaging")
			return
		}
		_, isonline2 := onlineusers[uid]
		if !isonline2 {
			log.Println("Offline user trying to send commands: " + uid)
			return
		}

		//TODO config'ed out
		dur24h, _ := time.ParseDuration("24h")
		dur30s, _ := time.ParseDuration("30s")

		//create private room, auto-invite & join the two participants
		roomid := uid + ":" + muid
		roomid2 := muid + ":" + uid
		_, ok = rooms[roomid]
		_, ok2 := rooms[roomid2]
		if !ok && !ok2 {
			rooms[roomid] = createRoom(users[uid].Path("nick").Data().(string)+" / "+users[muid].Path("nick").Data().(string), roomid, true, uid, dur24h, dur30s, "", 100)
		} else if !ok && ok2 {
			roomid = roomid2
		}

		//ok got the room, send join to uids
		addToRoom(so, uid, roomid)
		addToRoom(*mu.So, muid, roomid)
	})

	so.On("createroom", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := extractAndCheckToken(so, g)
		if !ok {
			return
		}

		name := g.Path("roomname").Data().(string)
		isprivate := g.Path("isprivate").Data().(bool)
		minexptime := g.Path("minexptime").Data().(string)
		maxexptime := g.Path("maxexptime").Data().(string)

		dur24h, _ := time.ParseDuration("24h")
		dur30s, _ := time.ParseDuration("30s")

		mindur, err := time.ParseDuration(minexptime)
		if err != nil {
			mindur = dur30s
		}

		maxdur, err := time.ParseDuration(maxexptime)
		if err != nil {
			maxdur = dur24h
		}

		roomid := uuid.NewV4().String()

		for _, v := range rooms {
			if v.FriendlyName == name {
				so.Emit("auth_error", "Room already exists")
				return
			}
		}

		rooms[roomid] = createRoom(name, roomid, isprivate, uid, maxdur, mindur, "", 100)
		addToRoom(so, uid, roomid)
	})

	so.On("leave", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := extractAndCheckToken(so, g)
		if !ok {
			return
		}

		//TODO handle "leaving" a direct message room
		so.Emit("leave", g.Path("room").Data().(string))
		so.Leave(g.Path("room").Data().(string))
		broadcastUserLeave(g.Path("room").Data().(string), uid, so)
	})

	so.On("join", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := extractAndCheckToken(so, g)
		if !ok {
			return
		}

		room := ""
		if g.Path("roomid").Data() != nil {
			roomid := g.Path("roomid").Data().(string)
			_, ok := rooms[roomid]
			if ok {
				room = roomid
			}
		} else {
			roomname := g.Path("roomname").Data().(string)
			for k, v := range rooms {
				if v.FriendlyName == roomname {
					room = k
					break
				}
			}
		}

		if room == "" {
			so.Emit("auth_error", "Room not found")
			return
		}

		_, inroom := rooms[room].MemberUIDs[uid]
		if !inroom && canJoin(uid, room, true) {
			addToRoom(so, uid, room)
		} else if inroom {
			sendRoomHistory(so, uid, room)
		} else {
			so.Emit("auth_error", "You can't join this room")
		}
	})

	so.On("deletechatm", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := extractAndCheckToken(so, g)
		if !ok {
			return
		}

		room := g.Path("room").Data().(string)
		msgid := g.Path("msgid").Data().(string)

		for i := 0; i < len(rooms[room].Messages); i++ {
			m := rooms[room].Messages[i]
			if m.Path("msgid").Data().(string) == msgid {
				if m.Path("uid").Data().(string) == uid {
					so.Emit("deletechatm", msgid)
					so.BroadcastTo(room, "deletechatm", msgid)
					rooms[room].Messages = append(rooms[room].Messages[:i], rooms[room].Messages[i+1:]...)
				} else {
					so.Emit("auth_error", "Invalid UID, not your message")
				}
				return
			}
		}
	})

	so.On("chatm", func(msg string) {
		//log.Println(msg)
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := extractAndCheckToken(so, g)
		if !ok {
			so.Emit("auth_error", "Invalid Room")
			return
		}

		//TODO message size limit enforcement

		g.SetP("", "t")    //clear full token
		g.SetP(uid, "uid") //set uid and user info
		g.SetP(time.Now().Unix(), "time")
		g.SetP(users[uid].Path("nick").Data().(string), "nick")
		g.SetP(uuid.NewV4().String(), "msgid")
		g.SetP(users[uid].Path("avatar").Data().(string), "avatar")

		//validate if user is in this room
		if g.Path("room").Data() == nil {
			so.Emit("auth_error", "Invalid Room: "+msg)
			return
		}
		roomtmp := g.Path("room").Data().(string)
		room := ""
		inrooms := so.Rooms()
		for i := 0; i < len(inrooms); i++ {
			if inrooms[i] == roomtmp {
				room = roomtmp
			}
		}
		if room == "" {
			so.Emit("auth_error", "Invalid Room")
			return
		}

		g.SetP(rooms[room].FriendlyName, "name")

		//validate duration length
		dur, err := time.ParseDuration(g.Path("dur").Data().(string))
		if err != nil {
			g.SetP(strconv.Itoa(int(rooms[room].MaxExpTime.Seconds()))+"s", "dur")
		} else {
			if dur > rooms[room].MaxExpTime {
				dur = rooms[room].MaxExpTime
				g.SetP(strconv.Itoa(int(dur.Seconds()))+"s", "dur")
			}
			if dur < rooms[room].MinExpTime {
				dur = rooms[room].MinExpTime
				g.SetP(strconv.Itoa(int(dur.Seconds()))+"s", "dur")
			}
		}

		rooms[room].Messages = append(rooms[room].Messages, g)
		so.Emit("chatm", g.String())
		so.BroadcastTo(room, "chatm", g.String())
	})
}

func sendOnlineUserList(so socketio.Socket) {
	cu, _ := gabs.ParseJSON([]byte("{}"))
	uids := []string{}
	nicks := []string{}
	for k := range onlineusers {
		uids = append(uids, k)
		nicks = append(nicks, users[k].Path("nick").Data().(string))
	}
	cu.SetP(uids, "uids")
	cu.SetP(nicks, "nicks")

	so.Emit("onlineusers", cu.String())
}

func broadcastUserLeave(room string, uid string, so socketio.Socket) {
	bc, _ := gabs.ParseJSON([]byte("{}"))
	bc.SetP(users[uid].Path("uid").Data().(string), "uid")
	bc.SetP(room, "room")
	so.BroadcastTo(room, "leave", bc.String())
	so.Emit("leave", bc.String())
}

func createRoomList() string {
	list, _ := gabs.ParseJSON([]byte("{}"))

	for k, v := range rooms {
		if !v.IsPrivate {
			list.SetP(v.FriendlyName, k)
		}
	}

	return list.String()
}

func onlineUserTimeout(server *socketio.Server) {
	d, _ := time.ParseDuration("300s") //5 minute timeout
	for {
		for k, v := range onlineusers {
			diff := time.Now().Sub(v.LastAct)
			if diff > d {
				delete(onlineusers, k)

				rms := (*v.So).Rooms()
				for i := 0; i < len(rms); i++ {
					broadcastUserLeave(rms[i], k, *v.So)
				}

				(*v.So).BroadcastTo("lobby", "userdisconnect", `{"uid":"`+k+`"}`)
			}
		}

		time.Sleep(d)
	}
}

func expireHistory(server *socketio.Server) {
	d, _ := time.ParseDuration("30s")
	for {
		//log.Println("Running history expiration")
		for k, v := range rooms {
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

func canJoin(uid string, room string, removeinvite bool) bool {
	if !rooms[room].IsPrivate {
		return true
	}

	_, ok := rooms[room].InvitedUIDs[uid]
	if ok {
		delete(rooms[room].InvitedUIDs, uid)
		return true
	}
	if rooms[room].CreatorUID == uid {
		return true
	}
	return false
}

func joinRooms(so socketio.Socket, uid string) {
	for k, v := range rooms {
		_, ok := v.MemberUIDs[uid]
		if ok {
			joinRoom(so, uid, k)
			//log.Println(h.String())
		}
	}
}

func addToRoom(so socketio.Socket, uid string, room string) {
	rooms[room].MemberUIDs[uid] = uid
	if so != nil {
		joinRoom(so, uid, room)
	}
}

func joinRoom(so socketio.Socket, uid string, room string) {
	v := rooms[room]
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
	bc.SetP(users[uid].Path("nick").Data().(string), "nick")
	bc.SetP(uid, "uid")
	bc.SetP(k, "room")
	bc.SetP(v.FriendlyName, "name")
	so.BroadcastTo(k, "joined", bc.String())

	sendRoomHistory(so, uid, room)
}

func sendRoomHistory(so socketio.Socket, uid string, room string) {
	v := rooms[room]
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

func createRoom(fname, roomid string, isprivate bool, creatoruid string, maxexptime time.Duration, minexptime time.Duration, avatar string, maxhistorylen int) *Room {
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
	return &r
}

/*func arrayContains(array []string, s string) (int, bool) {
	i := 0
	for j := 0; j < len(array); j++ {

	}

	return -1, false
}*/

func extractAndCheckToken(so socketio.Socket, g *gabs.Container) (string, bool) {
	if g.Path("t").Data() == nil {
		so.Emit("auth_error", "Missing Token")
		return "", false
	}

	uid, err := validateUserToken(nil, g.Path("t").Data().(string))
	if err != nil {
		so.Emit("auth_error", "Invalid Token")
		return "", false
	}

	return uid, true
}

func publicUserString(token *gabs.Container) string {
	s, err := gabs.ParseJSON([]byte(token.String()))
	if err != nil {
		return "{}"
	}

	s.SetP("", "privid")

	return s.String()
}

func validateUserToken(so socketio.Socket, msg string) (string, error) {
	s, err := base64.StdEncoding.DecodeString(msg)
	if err != nil {
		return "", err
	}
	byt := []byte(s)

	tok, err := decrypt(userkey, byt)
	if err != nil {
		return "", err
	}

	uncomp, err := uncompress(tok)
	if err != nil {
		return "", err
	}

	token, err := gabs.ParseJSON(uncomp.Bytes())
	if err != nil {
		return "", err
	}

	uid := token.Path("uid").Data().(string)
	bandate, isbanned := banlist[uid]
	if isbanned {
		return "", &Err{"Banned on " + bandate}
	}

	_, ok := users[uid]
	if !ok {
		users[uid] = token
		if so != nil {
			addToRoom(so, uid, "lobby")
		} else {
			rooms["lobby"].MemberUIDs[uid] = uid
		}
	} else {
		if users[uid].Path("privid").Data().(string) != token.Path("privid").Data().(string) {
			return "", &Err{"Bad privID"}
		}
	}

	return uid, nil
}

func login(w http.ResponseWriter, r *http.Request) {
	fc, _ := ioutil.ReadFile("./static/login.html")
	if fc != nil {
		fmt.Fprintf(w, string(fc[:]))
	}
}

func signup(w http.ResponseWriter, r *http.Request) {
	//TODO Validation
	if strings.ToUpper(r.Method) == "POST" {
		invite, ok := invites[r.FormValue("invite")]

		if ok {
			delete(invites, r.FormValue("invite"))
			token, _ := createNewUserToken(
				r.FormValue("nick"),
				r.FormValue("name"),
				r.FormValue("email"),
				r.FormValue("phone"),
				r.FormValue("url"),
				r.FormValue("desc"),
				r.FormValue("avatar"),
				r.FormValue("alertaddress"))

			users[token.Path("uid").Data().(string)] = token

			competok := compress([]byte(token.String()))
			etok, _ := encrypt(userkey, competok.Bytes())

			addToRoom(nil, token.Path("uid").Data().(string), "lobby")

			//TODO use templates
			fmt.Fprintf(w, `<html>`)
			fmt.Fprintf(w, `<strong>A message from your invite: </strong>`+invite+`<br><br>`)
			fmt.Fprintf(w, "Token (KEEP THIS SOMEWHERE SAFE OR SAVE THE LOGIN LINK): <br><textarea rows='10' cols='60'>%s</textarea><br><br>", base64.StdEncoding.EncodeToString(etok))
			fmt.Fprintf(w, "<a href='/#%s'>Assemble Chat Login</a> BOOKMARK THIS! <strong>DO NOT SHARE THIS LINK</strong>", base64.StdEncoding.EncodeToString(etok))
			fmt.Fprintf(w, `</html>`)
		} else {
			fmt.Fprintf(w, `Invalid Invite ID`)
		}
	} else {
		fc, _ := ioutil.ReadFile("./static/signup.html")
		if fc != nil {
			fmt.Fprintf(w, string(fc[:]))
		}
	}
}

func createNewUserToken(nick string, name string, email string, phone string, url string, desc string, avatar string, alertaddress string) (*gabs.Container, error) {

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

func encrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return ciphertext, nil
}

func decrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)
	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func compress(data []byte) bytes.Buffer {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	w.Write(data)
	w.Close()
	return b
}

func uncompress(data []byte) (bytes.Buffer, error) {
	var buf bytes.Buffer
	buf.Write(data)

	var unb bytes.Buffer
	r, err := zlib.NewReader(&buf)
	if err != nil {
		return unb, err
	}
	io.Copy(&unb, r)
	r.Close()
	return unb, nil
}
