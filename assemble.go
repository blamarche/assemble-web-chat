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
package main

// This is not meant to be clean from the get-go and I'm sure it'll need refactoring
//TODO investigate first-refresh join of lobby, etc

import (
	"encoding/base64"
	"fmt"
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

	"github.com/blamarche/assemble-web-chat/assemble"
	"github.com/blamarche/assemble-web-chat/config"
	"github.com/blamarche/assemble-web-chat/utils"
)

var service *assemble.Service

//MAIN FUNC
func main() {
	cfg, _ := config.DefaultConfig()
	defaultinvite := ""

	//read params from terminal
	if len(os.Args) > 1 {
		cfgfile := os.Args[1]
		if cfgfile == "--help" {
			fmt.Println("usage: assemble <path/config.json> <invitekey>")
			return
		}

		// Read the custom config
		clientconfig, err := ioutil.ReadFile(cfgfile)
		if err != nil {
			log.Fatal("Error reading config.json:", err)
		}
		cfg, err = config.LoadConfig(cfg, string(clientconfig))
		if err != nil {
			log.Fatal("Error parsing config.json:", err)
		}

		if len(os.Args) > 2 {
			defaultinvite = os.Args[2]
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
	err := httpscerts.Check("cert.pem", "key.pem")
	if err != nil {
		err = httpscerts.Generate("cert.pem", "key.pem", cfg.Host)
		if err != nil {
			log.Fatal("Error: Couldn't find or create create https certs.")
		}
	}

	service.SocketServer.On("connection", socketHandlers)

	service.SocketServer.On("error", func(so socketio.Socket, err error) {
		log.Println("socket error:", err)
	})

	http.Handle("/socket.io/", service.SocketServer)
	http.HandleFunc("/signup/", signupHandler)
	http.HandleFunc("/login/", loginHandler)
	http.Handle("/", http.FileServer(http.Dir("./static")))

	service.Start()
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	fc, _ := ioutil.ReadFile("./static/login.html")
	if fc != nil {
		fmt.Fprintf(w, string(fc[:]))
	}
}

func signupHandler(w http.ResponseWriter, r *http.Request) {
	//TODO Validation
	if strings.ToUpper(r.Method) == "POST" {
		invite, ok := service.Invites[r.FormValue("invite")]

		if ok {
			delete(service.Invites, r.FormValue("invite"))
			token, _ := assemble.CreateNewUserToken(
				r.FormValue("nick"),
				r.FormValue("name"),
				r.FormValue("email"),
				r.FormValue("phone"),
				r.FormValue("url"),
				r.FormValue("desc"),
				r.FormValue("avatar"),
				r.FormValue("alertaddress"))

			service.Users[token.Path("uid").Data().(string)] = token

			competok := utils.Compress([]byte(token.String()))
			etok, _ := utils.Encrypt(service.UserKey, competok.Bytes())

			service.AddToRoom(nil, token.Path("uid").Data().(string), "lobby")

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

		//add user to online status
		ou := assemble.OnlineUser{&so, time.Now()}
		service.OnlineUsers[uid] = &ou
		//since the user end sup joining lobby anyway, no need to broadcast any global online status alert

		so.On("disconnection", func() {
			//send disconnect message to all rooms they are in
			for k, v := range service.OnlineUsers {
				if v.So == &so {
					delete(service.OnlineUsers, k)

					rms := so.Rooms()
					for i := 0; i < len(rms); i++ {
						service.BroadcastUserLeave(rms[i], k, so)
					}

					so.BroadcastTo("lobby", "userdisconnect", `{"uid":"`+k+`"}`)
					break
				}
			}
		})

		service.JoinRooms(so, uid)
	})

	so.On("inviteusertoroom", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			return
		}

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

				name := service.Rooms[room].FriendlyName
				(*invuser.So).Emit("inviteusertoroom", `{"room":"`+room+`", "name":"`+name+`"}`)
			}
		} else {
			so.Emit("auth_error", "Invalid uid")
		}
	})

	so.On("userinfo", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		_, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			return
		}

		uid := g.Path("uid").Data().(string)
		u, uok := service.Users[uid]
		if uok {
			so.Emit("userinfo", assemble.PublicUserString(u))
		} else {
			so.Emit("auth_error", "Invalid uid")
		}
	})

	so.On("onlineusers", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		_, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			return
		}

		service.SendOnlineUserList(so)
	})

	so.On("ping", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			return
		}

		//TODO optimize for network performance
		_, ok2 := service.OnlineUsers[uid]
		if ok2 {
			service.OnlineUsers[uid].LastAct = time.Now()
		}
	})

	so.On("invitenewuser", func(msg string) {
		//log.Println(msg)
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			return
		}

		em := g.Path("email").Data().(string)
		fmt.Println(uid, "invited", em)

		//TODO email invite
		id := uuid.NewV4().String()
		service.Invites[id] = em

		so.Emit("invitenewuser", "{\"key\": \""+id+"\"}")
	})

	so.On("roomlist", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		_, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			return
		}

		so.Emit("roomlist", service.CreateRoomList())
	})

	so.On("ban", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		_, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			return
		}

		//TODO save/load bans to disk
		pass := g.Path("pass").Data().(string)
		banid := g.Path("uid").Data().(string)

		if pass == service.Cfg.AdminPass {
			service.Banlist[banid] = time.Now().String()
			so.Emit("ban", "Banned")
		} else {
			so.Emit("auth_error", "Bad admin password")
		}
	})

	so.On("unban", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		_, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			return
		}

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
	})

	so.On("directmessage", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			return
		}

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

		//create private room, auto-invite & join the two participants
		roomid := uid + ":" + muid
		roomid2 := muid + ":" + uid
		_, ok = service.Rooms[roomid]
		_, ok2 := service.Rooms[roomid2]
		if !ok && !ok2 {
			service.CreateRoom(service.Users[uid].Path("nick").Data().(string)+" / "+service.Users[muid].Path("nick").Data().(string), roomid, true, uid, service.DefMaxExp, service.DefMinExp, "", 100)
		} else if !ok && ok2 {
			roomid = roomid2
		}

		//ok got the room, send join to uids
		service.AddToRoom(so, uid, roomid)
		service.AddToRoom(*mu.So, muid, roomid)
	})

	so.On("createroom", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			return
		}

		name := g.Path("roomname").Data().(string)
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

		for _, v := range service.Rooms {
			if v.FriendlyName == name {
				so.Emit("auth_error", "Room already exists")
				return
			}
		}

		service.CreateRoom(name, roomid, isprivate, uid, maxdur, mindur, "", 100)
		service.AddToRoom(so, uid, roomid)
	})

	so.On("leave", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			return
		}

		//TODO handle "leaving" a direct message room
		so.Emit("leave", g.Path("room").Data().(string))
		so.Leave(g.Path("room").Data().(string))
		service.BroadcastUserLeave(g.Path("room").Data().(string), uid, so)
	})

	so.On("join", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			return
		}

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
		} else if inroom {
			service.SendRoomHistory(so, uid, room)
		} else {
			so.Emit("auth_error", "You can't join this room")
		}
	})

	so.On("deletechatm", func(msg string) {
		g, _ := gabs.ParseJSON([]byte(msg))

		uid, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			return
		}

		room := g.Path("room").Data().(string)
		msgid := g.Path("msgid").Data().(string)

		for i := 0; i < len(service.Rooms[room].Messages); i++ {
			m := service.Rooms[room].Messages[i]
			if m.Path("msgid").Data().(string) == msgid {
				if m.Path("uid").Data().(string) == uid {
					so.Emit("deletechatm", msgid)
					so.BroadcastTo(room, "deletechatm", msgid)
					service.Rooms[room].Messages = append(service.Rooms[room].Messages[:i], service.Rooms[room].Messages[i+1:]...)
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

		uid, ok := service.ExtractAndCheckToken(so, g)
		if !ok {
			so.Emit("auth_error", "Invalid Room")
			return
		}

		//TODO message size limit enforcement

		g.SetP("", "t")    //clear full token
		g.SetP(uid, "uid") //set uid and user info
		g.SetP(time.Now().Unix(), "time")
		g.SetP(service.Users[uid].Path("nick").Data().(string), "nick")
		g.SetP(uuid.NewV4().String(), "msgid")
		g.SetP(service.Users[uid].Path("avatar").Data().(string), "avatar")

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

		service.Rooms[room].Messages = append(service.Rooms[room].Messages, g)
		so.Emit("chatm", g.String())
		so.BroadcastTo(room, "chatm", g.String())
	})
}
