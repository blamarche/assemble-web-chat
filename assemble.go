/*
This file is part of Assemble Web Chat.

Assemble Web Chat is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Assemble Web Chat is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with Assemble Web Chat.  If not, see <http://www.gnu.org/licenses/>.
*/
package main

//This is not meant to be clean from the get-go and it will need refactoring

import (
	"encoding/base64"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Jeffail/gabs"
	"github.com/googollee/go-socket.io"
	"github.com/kabukky/httpscerts"
	"github.com/satori/go.uuid"

	"github.com/blamarche/assemble-web-chat/assemble-lib"
	"github.com/blamarche/assemble-web-chat/config"
	"github.com/blamarche/assemble-web-chat/utils"
)

var service *assemble.Service

//MAIN FUNC
func main() {
	cfg, _ := config.DefaultConfig()
	defaultinvite := ""

	//read params from terminal
	cfgfile := "./config.json"
	if len(os.Args) > 1 {
		cfgfile = os.Args[1]
		if cfgfile == "--help" {
			fmt.Println("usage: assemble <path/config.json> <invitekey>")
			return
		}

		if len(os.Args) > 2 {
			defaultinvite = os.Args[2]
		}
	}

	// Read the custom config
	clientconfig, err := ioutil.ReadFile(cfgfile)
	if err != nil {
		log.Println("Error reading config.json:", err)
	} else {
		cfg, err = config.LoadConfig(cfg, string(clientconfig))
		if err != nil {
			log.Fatal("Error parsing config.json:", err)
		}
	}

	//grab/create enc key
	userkey := []byte{}
	fc, _ := ioutil.ReadFile("./userkey.txt")
	if fc != nil {
		userkey = fc[:]
	} else {
		tmp := uuid.NewV4().String()
		tmp = tmp[4:]
		ioutil.WriteFile("./userkey.txt", []byte(tmp), 0666)
		userkey = []byte(tmp)
	}

	service = assemble.NewService(cfg, userkey)

	if defaultinvite != "" {
		service.Invites[defaultinvite] = "admin@localhost"
	}

	// Check if the cert files are available and make new ones if needed
	err = httpscerts.Check("cert.pem", "key.pem")
	if err != nil {
		err = httpscerts.Generate("cert.pem", "key.pem", cfg.Host)
		if err != nil {
			log.Fatal("Error: Couldn't find or create https certs.")
		}
	}

	service.SocketServer.On("connection", socketHandlers)
	service.SocketServer.On("error", func(so socketio.Socket, err error) {
		log.Println("socket error:", err)
	})

	http.Handle("/socket.io/", service.SocketServer)
	http.HandleFunc("/signup/", signupHandler)
	http.HandleFunc("/login/", loginHandler)
	http.HandleFunc("/iconlib.js", iconlibHandler)
	http.Handle("/", http.FileServer(http.Dir("./static")))

	service.Start()
}

func iconlibHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, service.IconsJs)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	fc, _ := ioutil.ReadFile("./static/login.html")
	if fc != nil {
		fmt.Fprintf(w, string(fc[:]))
	}
}

// signupHandler controls the signup and user profile update forms
func signupHandler(w http.ResponseWriter, r *http.Request) {
	if strings.ToUpper(r.Method) == "POST" {
		invite, ok := service.Invites[r.FormValue("invite")]
		tokenenc := r.FormValue("token")

		var token *gabs.Container

		if ok {
			delete(service.Invites, r.FormValue("invite"))
			token, _ = assemble.CreateNewUserToken(
				r.FormValue("nick"),
				r.FormValue("name"),
				r.FormValue("email"),
				r.FormValue("phone"),
				r.FormValue("url"),
				r.FormValue("desc"),
				r.FormValue("avatar"),
				r.FormValue("alertaddress"))
		} else if tokenenc != "" {
			uid, tokerr := service.ValidateUserToken(nil, tokenenc)
			if tokerr == nil {
				privid := service.Users[uid].Token.Path("privid").Data().(string)
				token, _ = assemble.CreateUpdatedUserToken(
					r.FormValue("nick"),
					r.FormValue("name"),
					r.FormValue("email"),
					r.FormValue("phone"),
					r.FormValue("url"),
					r.FormValue("desc"),
					r.FormValue("avatar"),
					r.FormValue("alertaddress"),
					uid, privid)

				log.Println(uid, "updated user token")
			} else {
				tokenenc = ""
			}
		}

		if ok || tokenenc != "" {

			competok := utils.Compress([]byte(token.String()))
			etok, _ := utils.Encrypt(service.UserKey, competok.Bytes())

			service.AddToRoom(nil, token.Path("uid").Data().(string), "lobby")

			//TODO use templates
			fmt.Fprintf(w, `<html>`)
			fmt.Fprintf(w, `<meta http-equiv="refresh" content="10; url=/#%s">`, base64.StdEncoding.EncodeToString(etok))
			if ok {
				fmt.Fprintf(w, `<strong>SUCCESS!</strong> `+invite+`<br><br>`)
			} else {
				fmt.Fprintf(w, `<strong>Delete your old login bookmark and close the window or you will still be using your old profile!</strong><br><br>`)
			}
			//fmt.Fprintf(w, "Token (KEEP THIS SOMEWHERE SAFE OR SAVE THE LOGIN LINK): <br><textarea rows='10' cols='60'>%s</textarea><br><br>", base64.StdEncoding.EncodeToString(etok))
			fmt.Fprintf(w, "<a href='/#%s' target='_blank'>Assemble Chat Login - CLICK HERE AND BOOKMARK THE CHAT!</a> <strong>DO NOT SHARE THIS LINK</strong>", base64.StdEncoding.EncodeToString(etok))
			fmt.Fprintf(w, "<br>You will automatically enter the chat in 10 seconds...")
			fmt.Fprintf(w, `</html>`)
		} else {
			fmt.Fprintf(w, `Invalid Invite ID or Token`)
		}
	} else {
		fc, _ := ioutil.ReadFile("./static/signup.html")
		if fc != nil {
			fmt.Fprintf(w, string(fc[:]))
		}
	}
}

type soHandler func(string)
type soHandlerJSON func(string, *gabs.Container)

func jsonSocketWrapper(so socketio.Socket, checkUser bool, f soHandlerJSON) soHandler {
	return soHandler(func(msg string) {
		defer func() {
			if err := recover(); err != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				log.Printf("socket error %v: %s", err, buf)
			}
		}()

		g, err := gabs.ParseJSON([]byte(msg))
		if err != nil {
			log.Println(err)
			return
		}

		uid := ""
		ok := false
		if checkUser {
			uid, ok = service.ExtractAndCheckToken(so, g)
			if !ok {
				return
			}
			service.Users[uid].LastAct = time.Now()
		}

		f(uid, g)
	})
}

//TODO Refactor socket handlers, move logic into assemble.Service
func socketHandlers(so socketio.Socket) {
	so.On("auth", func(msg string) {
		uid, err := service.ValidateUserToken(nil, msg)
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
		service.SendOnlineUserList(so)
		so.Emit("roomlist", service.CreateRoomList())

		//add user to online status
		service.SetUserOnline(uid, so)
		//since the user end sup joining lobby anyway, no need to broadcast any global online status alert

		so.On("disconnection", func() {
			//send disconnect message to all rooms they are in
			for k, v := range service.OnlineUsers {
				if v.So == &so {
					delete(service.OnlineUsers, k)
					service.Users[k].LastAlert = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
					service.Users[k].LastAct = time.Now()

					rms := so.Rooms()
					for i := 0; i < len(rms); i++ {
						service.BroadcastUserLeave(rms[i], k, so)
					}

					so.BroadcastTo("lobby", "userdisconnect", `{"uid":"`+k+`"}`)
					log.Println("disconnected", k)
					break
				}
			}
		})

		service.JoinRooms(so, uid)
	})

	so.On("inviteusertoroom", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		invuid := g.Path("uid").Data().(string)
		_, uok := service.Users[invuid]
		if uok {
			invuser, invok := service.OnlineUsers[invuid]
			if !invok {
				so.Emit("auth_error", "Offline uid, try again later")
				return
			}

			room := g.Path("room").Data().(string)
			r, rok := service.Rooms[room]
			if rok {
				if r.IsPrivate {
					caninvite := false
					if r.CreatorUID == uid {
						caninvite = true
					} else {
						_, ismember := r.MemberUIDs[uid]
						caninvite = ismember
					}

					if !caninvite {
						so.Emit("auth_error", "You aren't a member of that room!")
						return
					}

					//finally passed the tests... lets add to the invite list of uids
					r.InvitedUIDs[invuid] = time.Now().String()
				}

				name := service.Rooms[room].FriendlyName
				(*invuser.So).Emit("inviteusertoroom", `{"room":"`+room+`", "name":"`+name+`"}`)
			}
		} else {
			so.Emit("auth_error", "Invalid uid")
		}
	}))

	so.On("userinfo", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		uid = g.Path("uid").Data().(string)
		u, uok := service.Users[uid]
		if uok {
			so.Emit("userinfo", assemble.PublicUserString(u.Token))
		} else {
			so.Emit("auth_error", "Invalid uid")
		}
	}))

	so.On("onlineusers", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		service.SendOnlineUserList(so)
	}))

	so.On("roomusers", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		//check room, check uid in room
		room := g.Path("room").Data().(string)
		r, rok := service.Rooms[room]
		if rok {
			_, ismember := r.MemberUIDs[uid]
			if ismember {
				service.SendRoomUserList(so, room)
			}
		}
	}))

	so.On("ping", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		//TODO optimize for network performance
		_, ok2 := service.OnlineUsers[uid]
		if ok2 {
			service.OnlineUsers[uid].LastPing = time.Now()
		} else {
			service.SetUserOnline(uid, so)
		}
	}))

	so.On("invitenewuser", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		if g.Path("email").Data() != nil {
			em := g.Path("email").Data().(string)
			fmt.Println(uid, "invited", html.EscapeString(em))

			id := uuid.NewV4().String()
			service.Invites[id] = em

			so.Emit("invitenewuser", "{\"key\": \""+id+"\"}")
		} else {
			so.Emit("auth_error", "Invalid data received. Try again.")
		}
	}))

	so.On("roomlist", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		so.Emit("roomlist", service.CreateRoomList())
	}))

	so.On("ban", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		//TODO save/load bans to disk
		pass := g.Path("pass").Data().(string)
		banid := g.Path("uid").Data().(string)

		if pass == service.Cfg.AdminPass {
			service.Banlist[banid] = time.Now().String()
			so.Emit("ban", "Banned")
		} else {
			so.Emit("auth_error", "Bad admin password")
		}
	}))

	so.On("unban", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		//TODO save/load bans to disk
		pass := g.Path("pass").Data().(string)
		banid := g.Path("uid").Data().(string)

		if pass == service.Cfg.AdminPass {
			delete(service.Banlist, banid)
			so.Emit("ban", "Unbanned")
		} else {
			so.Emit("auth_error", "Bad admin password")
			return
		}
	}))

	so.On("directmessage", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		muid := g.Path("uid").Data().(string)

		//check users
		mu, isonline := service.OnlineUsers[muid]
		if !isonline {
			so.Emit("auth_error", "User must be online to initiate direct messaging")
			return
		}
		_, isonline2 := service.OnlineUsers[uid]
		if !isonline2 {
			log.Println("Offline user trying to send commands: " + uid)
			return
		}

		if uid == muid {
			so.Emit("auth_error", "You can't message yourself")
			return
		}

		//create private room, auto-invite & join the two participants
		roomid := uid + ":" + muid
		roomid2 := muid + ":" + uid
		_, ok := service.Rooms[roomid]
		_, ok2 := service.Rooms[roomid2]
		if !ok && !ok2 {
			service.CreateRoom(service.Users[uid].Token.Path("nick").Data().(string)+" / "+service.Users[muid].Token.Path("nick").Data().(string), roomid, true, true, uid, service.DefMaxExp, service.DefMinExp, "", service.Cfg.MaxHistoryLen)
			service.Rooms[roomid].DirectUIDs[uid] = uid
			service.Rooms[roomid].DirectUIDs[muid] = muid
		} else if !ok && ok2 {
			roomid = roomid2
		}

		//ok got the room, send join to uids
		service.AddToRoom(so, uid, roomid)
		service.AddToRoom(*mu.So, muid, roomid)
	}))

	so.On("createroom", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		name := html.EscapeString(g.Path("roomname").Data().(string))
		isprivate := g.Path("isprivate").Data().(bool)
		minexptime := g.Path("minexptime").Data().(string)
		maxexptime := g.Path("maxexptime").Data().(string)

		dur24h := service.DefMaxExp
		dur30s := service.DefMinExp

		mindur, err := time.ParseDuration(minexptime)
		if err != nil {
			mindur = dur30s
		}

		maxdur, err := time.ParseDuration(maxexptime)
		if err != nil {
			maxdur = dur24h
		}

		roomid := uuid.NewV4().String()
		name = strings.Trim(name, " \n\t")
		if name == "" {
			so.Emit("auth_error", "Room must have a name")
			return
		}

		for _, v := range service.Rooms {
			if v.FriendlyName == name {
				so.Emit("auth_error", "Room already exists")
				return
			}
		}

		service.CreateRoom(name, roomid, isprivate, false, uid, maxdur, mindur, "", service.Cfg.MaxHistoryLen)
		service.AddToRoom(so, uid, roomid)
	}))

	so.On("leave", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		//remove from members list, unless its lobby
		room := g.Path("room").Data().(string)
		_, roomok := service.Rooms[room]
		if roomok && room != "lobby" {
			delete(service.Rooms[room].MemberUIDs, uid)
		}
		so.Emit("leave", room)
		so.Leave(room)
		service.BroadcastUserLeave(room, uid, so)
	}))

	so.On("history", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		if g.Path("last").Data() == nil {
			service.SendRoomHistory(so, uid, g.Path("room").Data().(string), 15)
		} else {
			service.SendRoomHistory(so, uid, g.Path("room").Data().(string), int(g.Path("last").Data().(float64)))
		}
	}))

	so.On("join", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {

		room := ""
		if g.Path("roomid").Data() != nil {
			roomid := g.Path("roomid").Data().(string)
			_, ok := service.Rooms[roomid]
			if ok {
				room = roomid
			}
		} else {
			roomname := g.Path("roomname").Data().(string)
			for k, v := range service.Rooms {
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

		_, inroom := service.Rooms[room].MemberUIDs[uid]
		if !inroom && service.CanJoin(uid, room, true) {
			service.AddToRoom(so, uid, room)
		} else if !inroom {
			so.Emit("auth_error", "You can't join this room")
		}
	}))

	so.On("deletechatm", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		room := g.Path("room").Data().(string)
		msgid := g.Path("msgid").Data().(string)

		for i := 0; i < len(service.Rooms[room].Messages); i++ {
			m := service.Rooms[room].Messages[i]
			if m.Path("msgid").Data().(string) == msgid {
				if m.Path("uid").Data().(string) == uid {
					so.Emit("deletechatm", msgid)
					so.BroadcastTo(room, "deletechatm", msgid)
					service.Rooms[room].Messages = append(service.Rooms[room].Messages[:i], service.Rooms[room].Messages[i+1:]...)
				}
				return
			}
		}
	}))

	so.On("setalerts", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		enable := g.Path("enabled").Data().(bool)
		service.Users[uid].AlertsEnabled = enable
		if enable {
			so.Emit("setalerts", "Alerts enabled")
		} else {
			so.Emit("setalerts", "Alerts disabled")
		}
		log.Println(uid, "set alerts =", enable)
	}))

	so.On("chatm", jsonSocketWrapper(so, true, func(uid string, g *gabs.Container) {
		//add to online status if offline
		_, isonline := service.OnlineUsers[uid]
		if !isonline {
			service.SetUserOnline(uid, so)
		}

		g.SetP("", "t")    //clear full token
		g.SetP(uid, "uid") //set uid and user info
		g.SetP(time.Now().Unix(), "time")
		g.SetP(service.Users[uid].Token.Path("nick").Data().(string), "nick")
		g.SetP(uuid.NewV4().String(), "msgid")
		g.SetP(service.Users[uid].Token.Path("avatar").Data().(string), "avatar")

		if len(g.Path("m").Data().(string)) > 4096 {
			//TODO add exception for data image uris
			//so.Emit("auth_error", "Message is too long. Must be less than 4096 characters")
			//return
		}
		g.SetP(html.EscapeString(g.Path("m").Data().(string)), "m")

		//validate if user is in this room
		if g.Path("room").Data() == nil {
			so.Emit("auth_error", "Invalid Room: "+g.String())
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

		g.SetP(service.Rooms[room].FriendlyName, "name")

		//validate duration length
		dur, err := time.ParseDuration(g.Path("dur").Data().(string))
		if err != nil {
			g.SetP(strconv.Itoa(int(service.Rooms[room].MaxExpTime.Seconds()))+"s", "dur")
		} else {
			if dur > service.Rooms[room].MaxExpTime {
				dur = service.Rooms[room].MaxExpTime
				g.SetP(strconv.Itoa(int(dur.Seconds()))+"s", "dur")
			}
			if dur < service.Rooms[room].MinExpTime {
				dur = service.Rooms[room].MinExpTime
				g.SetP(strconv.Itoa(int(dur.Seconds()))+"s", "dur")
			}
		}

		//auto-join directUIDs who arent in member list
		if service.Rooms[room].IsDirect {
			for _, dmuid := range service.Rooms[room].DirectUIDs {
				_, dmok := service.Rooms[room].MemberUIDs[dmuid]
				if !dmok {
					dmu, isonline := service.OnlineUsers[dmuid]
					if !isonline {
						so.Emit("auth_error", "Warning: User is not online")
					} else {
						service.AddToRoom(*dmu.So, dmuid, room)
					}
				}
			}
		}

		service.Rooms[room].Messages = append(service.Rooms[room].Messages, g)
		so.Emit("chatm", g.String())
		so.BroadcastTo(room, "chatm", g.String())
	}))
}
