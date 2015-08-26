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


$(document).ready(function(){
    $( window ).resize(function() {
        updateSidebar();
    });

    var myDropZone = new Dropzone("#imgupFile",{
    //$("#imgupFile").dropzone({
        url:"/",
        paramName: "file", // The name that will be used to transfer the file
        maxFilesize: 0.5, // MB
        previewsContainer: $("#preview")[0],
        clickable: true,
        maxFiles:1,
        acceptedFiles: "image/*",
        autoProcessQueue: false,
        thumbnail: function(file, imguri) {
            //console.log(imguri)
            socket.emit('chatm', JSON.stringify({"t": token, "room": cur_room, "m": imguri, "dur":cur_dur}));
        }
    });
    //disable the normal dialog from showing
    $("#imgupFile").on('click',function() {
        //return false;
    });
    $("#imgup").on('click',function() {
        $("#imgupFile").click();
    });

    sendPing();
    function sendPing() {
        socket.emit("ping", JSON.stringify({"t": token}));
        setTimeout(sendPing, 149000);//~2.5 minutes. User timeouts every 5 minutes
        //TODO bandwidth optimize this! probably no need to send a full on token each time.
    }

    $("#messages").on('click', '.userprofilelink', function(e) {
        socket.emit("userinfo", JSON.stringify({"t": token, "uid": $(e.currentTarget).attr("data-uid")}));
    });

    $("#inviteusertoroom").on('click', function(e) {
        var uid = $(e.currentTarget).attr("data-uid");
        socket.emit("inviteusertoroom", JSON.stringify({"t": token, "uid": uid, "room": cur_room}));
    });

    $("#createnewroom").on('click', function(e) {
        var name, isprivate, maxexptime, minexptime;
        name=$("#createroom .roomname").val();
        isprivate=$("#createroom .isprivate:checked").length;
        maxexptime=$("#createroom .maxexptime").val();
        minexptime=$("#createroom .minexptime").val();
        socket.emit("createroom", JSON.stringify({"t": token, "roomname": name, "maxexptime": maxexptime, "minexptime": minexptime, "isprivate": (isprivate!=0)}));

        $("#createroom .roomname").val("");
    });

    $("#sendmessage").on('click', function(e) {
        var uid = $(e.currentTarget).attr("data-uid");
        socket.emit("directmessage",JSON.stringify({"t":token, "uid": uid}));
    });

    $('#messages').on('click', 'a.joinroom', function(ev){
        var rm = $(ev.currentTarget).attr("data-room");
        socket.emit("join", JSON.stringify({"t": token, "roomid": rm}));
        if (typeof rooms[rm]!="undefined") {
            switchRoom(rm);
            updateSidebar();
        }
        return false;
    });

    $('#messageduration .currentduration').change(function(e) {
        cur_dur = $('#messageduration .currentduration').val();
    });
});

//setup notifications
if ("Notification" in window) {
    if (Notification.permission === "granted") {
    } else if (Notification.permission !== 'denied') {
        Notification.requestPermission(function (permission) {
            if (permission === "granted") {
            }
        });
    }
} else {
    window.Notification = {permission:"denied"};
}

var socket = io("", {reconnectionDelayMax:1000, reconnectionDelay:1000, timeout: 10000000, multiplex:false});
socket.io.on('connect_timeout', function(e) {
    console.log("socket timeout");
    console.log(e);
});
socket.io.on('connect_error', function(e) {
    console.log("socket error");
    console.log(e);
});
socket.io.on('reconnect_error', function(e) {
    console.log("socket reconnect error");
    console.log(e);
});

var rooms = {};
var roomnames = {};
var cur_room = "";
var cur_dur = "48h";
var token = window.location.hash.substring(1);

$(window).on('beforeunload', function(){
    socket.close();
});

$('form').submit(function(){
    var m = $('#m').val();
    if (m!=""){
        if (m[0]=="/") {
            handleCommand(socket,m);
        } else {
            socket.emit('chatm', JSON.stringify({"t": token, "room": cur_room, "m": m, "dur":cur_dur}));
            $('#m').val('');
            $("#m").prop('disabled', true);
        }
    }
    return false;
});

//menu buttons.
//TODO switch to bootstrap dialogs, not 'prompt'
$("#btnroomlist").on('click', function() {
    socket.emit("roomlist",JSON.stringify({"t": token}));
});
$("#btncreateroom").on('click', function() {
    //var roomname = prompt("Enter the room name: ");
    //socket.emit("createroom", JSON.stringify({"t": token, "roomname": roomname, "private":false}));
    $("#createroom").modal();
});
$("#btnduration").on('click', function() {
    $("#messageduration").modal();
    /*
    var m = prompt('Enter the desired default expiration time for your messages (30s, 1m, 10m, 24h, etc): ');
    if (m!=null) {
        var unit = m.substring(m.length-1);
        if (unit!="s"&&unit!="m"&&unit!="h") {
            alert("Invalid value.");
        } else {
            cur_dur = m;
        }
    }
    */
});
$("#btninvitenewuser").on('click', function() {
    socket.emit("invitenewuser", JSON.stringify({"t": token, "email": prompt("Enter a message for the invited user (optional):")}));
});
$("#btnuserlist").on('click', function() {
    socket.emit("onlineusers",JSON.stringify({"t": token}));
});

socket.on('connect', function(d) {
    socket.emit("auth", token);
});

socket.on('chatm', function(d){
    //console.log(d);
    d=JSON.parse(d);
    appendChatMessage(d.uid,d.room,d.name,d.nick,d.m,d.msgid,d.avatar,d.time);
    scrollToBottom();
    if ($("#m").prop('disabled')==true) {
        $("#m").focus();
    }
    $("#m").prop('disabled', false);

    try {
        if (Notification.permission==="granted" && (cur_room!=d.room || !document.hasFocus()) && d.m.indexOf("data:image/")!=0)
            var notification = new Notification("["+d.name+"] "+d.nick+": "+d.m.substring(0,256)+" (Assemble Chat)");
    }catch (err) {
        console.log(err);
    }

    if (cur_room!=d.room) {
        rooms[d.room].mcount++;
        updateSidebar();
    }
});

socket.on('leave', function(room) {
    delete rooms[room];
    //TODO is this delete working right?
    $("#sidebar li[data-room='"+room+"']").remove();
});

socket.on('inviteusertoroom', function(d) {
    var d=JSON.parse(d);
    var m = "<span class='prefix'>You've been invited to join </span>";
    m+="<a class='joinroom' data-room='"+d.room+"'>"+d.name+"</a>";
    m+="<div class='clearfloat'></div>";
    $('#messages').append($('<li>').html(m));

    scrollToBottom();
});

socket.on('userinfo', function(d) {
    var d=JSON.parse(d);

    $("#userprofile .avatar").html("<img src='"+d.avatar+"'></img>");
    $("#userprofile .nick").text(d.nick);
    $("#userprofile .name").text(d.name);
    $("#userprofile .email").text(d.email);
    $("#userprofile .phone").text(d.phone);
    $("#userprofile .url").text(d.url);
    $("#userprofile .desc").html(d.desc);
    $("#inviteusertoroom").attr("data-uid",d.uid);
    $("#sendmessage").attr("data-uid",d.uid);

    $("#userprofile").modal();
})

socket.on('onlineusers', function(d) {
    var d=JSON.parse(d);
    var m = "<span class='prefix'>Online Users: </span>";
    for (var i=0; i<d.uids.length; i++) {
        m+="<a class='userprofilelink onlineuser' data-uid='"+d.uids[i]+"'>"+d.nicks[i]+"</a>";
    }
    m+="<div class='clearfloat'></div>";
    $('#messages').append($('<li>').html(m));

    scrollToBottom();
});

socket.on('roomlist', function(d){
    var d=JSON.parse(d);
    var m = "<span class='prefix'>Room List: </span>";
    for (var k in d) {
        m+="<a class='joinroom' data-room='"+k+"'>"+d[k]+"</a>";
    }
    m+="<div class='clearfloat'></div>";
    $('#messages').append($('<li>').html(m));

    scrollToBottom();
});

socket.on('history', function(d){
    var d=JSON.parse(d);
    //console.log(d);
    for (var i=0; i<d.history.length; i++) {
        appendChatMessage(d.history[i].uid,d.room,d.name,d.history[i].nick,d.history[i].m,d.history[i].msgid, d.history[i].avatar,d.history[i].time);
    }

    updateSidebar();
    scrollToBottom();
});

socket.on('join', function(d){
    var d=JSON.parse(d);
    if (d.minexptime.indexOf("h0m")!=-1) {
        d.minexptime = d.minexptime.replace("0m","");
        d.minexptime = d.minexptime.replace("0s","");
    }
    if (d.minexptime.indexOf("m0s")!=-1)
        d.minexptime = d.minexptime.replace("0s","");
    if (d.maxexptime.indexOf("h0m")!=-1) {
        d.maxexptime = d.maxexptime.replace("0m","");
        d.maxexptime = d.maxexptime.replace("0s","");
    }
    if (d.maxexptime.indexOf("m0s")!=-1)
        d.maxexptime = d.maxexptime.replace("0s","");

    if (!(d.name in roomnames)) {
        $('#messages').append($('<li>').text("Joined "+d.name+" ("+d.minexptime+" - "+d.maxexptime+")"));
        rooms[d.room] = {users: [], messages: [], friendlyname: d.name, mcount: 0, minexptime: d.minexptime, maxexptime: d.maxexptime};
        roomnames[d.name] = d.room;
        if (d.room=="lobby")
            switchRoom(d.room);
    }
    updateSidebar();
});

socket.on('joined', function(d){
    var d=JSON.parse(d);
    $('#messages').append($('<li>').text(d.nick +" joined "+d.name));
    rooms[d.room].users.push({uid:d.uid, nick:d.nick});
});

socket.on('auth_error', function(d){
    $('#messages').append($('<li>').text("Error: "+d));
    if (d=="Invalid Token") {
        $('#messages').append($('<li>').html("<a class='signup' href='/signup'>Sign up with your Invite Code</a>"));
    }
    $("#m").prop('disabled', false);
});

socket.on('auth', function(d){
    $('#messages').append($('<li>').text("Logged in successfully"));
});

socket.on('invitenewuser', function(d){
    var d=JSON.parse(d);
    $('#messages').append($('<li>').text("Invite Key: "+d.key));
});

socket.on('deletechatm', function(d){
    $("#messages li[data-msgid='"+d+"']").html("<i>Removed message</i>");
});

function updateSidebar() {
    //update sidebar to hold message counts and list active chat rooms
    for (var r in rooms) {
        var rm = rooms[r];
        if ($("#sidebar li[data-room='"+r+"']").length == 0) {
            $("#sidebar").append($("<li>")
                .attr("data-room", r)
                .attr("title", rm.minexptime+" - "+rm.maxexptime)
                .html("<div>"+rm.friendlyname+"</div><span></span>")
                .on('click', function(ev) {
                    var rm = $(ev.currentTarget).attr("data-room");
                    switchRoom(rm);
                })
                .on('contextmenu', function(ev) {
                    var rm = $(ev.currentTarget).attr("data-room");
                    socket.emit("leave", JSON.stringify({"t": token, "room": rm}));
                })
            );
        }

        if (rm.mcount > 0) {
            $("#sidebar li[data-room='"+r+"'] span").html(rm.mcount);
        } else {
            $("#sidebar li[data-room='"+r+"'] span").text("");
        }
        if (r == cur_room) {
            $("#sidebar li.active").removeClass("active");
            $("#sidebar li[data-room='"+r+"']").addClass("active");
        }
    }
    $("#sidebar").css("height", (window.innerHeight-42)+"px");
}

function scrollToBottom() {
    $(window).scrollTop($('body')[0].scrollHeight);
}

function appendChatMessage(uid, room, roomname, nick, m, id, avatar, time) {
    var hide = "";
    if (room!=cur_room) {
        hide="hidden";
    }

    var tmp = $('<div>')
    tmp.text(m);
    m=tmp.text();
    //TODO needs escaping

    if (m.indexOf("data:image/")==0) {
        m = "<img class='autolink upload' src='"+m+"'></img>";
    } else {
        m = Autolinker.link(m, {
            stripPrefix: false,
            truncate: 30,
            className:"autolink",
            twitter:false,
            hashtag: false,
            replaceFn : function( autolinker, match ) {
                href = match.getAnchorHref();
                switch( match.getType() ) {
                    case 'url' :
                        if ( match.getUrl().indexOf( '.jpg' ) !== -1 ||
                             match.getUrl().indexOf( '.jpeg' ) !== -1 ||
                             match.getUrl().indexOf( '.png' ) !== -1 ||
                             match.getUrl().indexOf( '.gif' ) !== -1  )
                        {
                            return "<a href='"+href+"' target='_blank'>"+href+"</a><br><img src='"+href+"' class='autolink'></img>";
                        }
                        break;
                }
                return;
            }
        });
    }

    if (avatar=="" || typeof avatar=="undefined") {
        avatar = "";
    }

    if (typeof time=="undefined") {
        time="";
    } else {
        time = fuzzyTime(new Date(time*1000));
    }

    var avatarimg = "<img src='"+avatar+"' class='avatar'></img>";
    if (avatar=="") {
        avatarimg = "<span class='glyphicon glyphicon-user avatar'></span>";
    }

    $('#messages').append($('<li>')
        .html("<div class='useravatar'>"+avatarimg+"</div><div class='messagecontainer'><a data-uid='"+uid+"' class='userprofilelink nick'>"+nick+"</a> <span class='time'>"+time+"</span> <br><span class='messagetext'>"+m+"</span>")
        .attr("data-msgid", id)
        .attr("data-room", room)
        .attr("title",id)
        .addClass("chatmsg")
        .addClass(hide)
        .on("contextmenu",function(ev) {
            var msgid = $(ev.currentTarget).attr("data-msgid")
            requestDeleteMessage(msgid);
            return false;
        })
        /*
        .on("click", function(ev) {
            console.log("Show user info"); //TODO
        })
        */
    );
}

function requestDeleteMessage(msgid) {
    socket.emit("deletechatm", JSON.stringify({
        "t": token,
        "room": cur_room,
        "msgid": msgid
    }));
}

function switchRoom(room) {
    if (typeof rooms[room]=="undefined") {
        appendChatMessage("","lobby", "Lobby", "SYSTEM", "Unknown room", "");
    } else {
        cur_room = room;
    }

    $("#messages li.chatmsg").addClass("hidden");
    $("#messages li.chatmsg[data-room='"+cur_room+"']").removeClass("hidden");

    rooms[cur_room].mcount = 0;

    updateSidebar();
    scrollToBottom();
    $("#m").focus();
}

function switchRoomByName(roomname) {
    var new_room = roomnames[roomname];
    if (new_room==null || typeof roomnames[roomname]=="undefined") {
        appendChatMessage("","lobby", "Lobby", "SYSTEM", "Unknown room", "");
    } else {
        switchRoom(new_room);
    }
}

//TODO make a better fuzzer
function fuzzyTime( previous) {
    current = new Date();
    var msPerMinute = 60 * 1000;
    var msPerHour = msPerMinute * 60;
    var msPerDay = msPerHour * 24;
    var msPerMonth = msPerDay * 30;
    var msPerYear = msPerDay * 365;

    var elapsed = current - previous;

    if (elapsed < msPerMinute) {
        return "";
    } else if (elapsed < msPerHour) {
        return Math.round(elapsed/msPerMinute) + ' minutes ago';
    } else if (elapsed < msPerDay ) {
        return Math.round(elapsed/msPerHour ) + ' hours ago';
    } else if (elapsed < msPerMonth) {
        //return Math.round(elapsed/msPerDay) + ' days ago';
        return previous.toLocaleDateString()
    } else if (elapsed < msPerYear) {
        //return Math.round(elapsed/msPerMonth) + ' months ago';
        return previous.toLocaleDateString()
    } else {
        //return Math.round(elapsed/msPerYear ) + ' years ago';
        return previous.toLocaleDateString()
    }
}

function handleCommand(socket,c) {
    var ca = c.split(" ");
    switch (ca[0]) {
        case "/help":
            $('#messages').append($('<li>').html(" \
                /leave - Leaves the current room <br>\
                /ban admin uid - Bans the UID permanently <br>\
                /unban admin uid - UnBans the UID <br>\
                /dur message-duration - Sets your message expiration time (ie: 24h, 10m, 30s, etc) <br>\
                /join room-name - Attempts to join a room by name <br>\
                /switch room-name - Switches your chat focus to a room by name <br>\
                /roomlist - Lists all public rooms with links to join <br>\
            "));
            break;
        case "/leave":
            socket.emit("leave", JSON.stringify({"t": token, "room": cur_room}));
            break;
        case "/ban":
            var pass = ca[1];
            var uid = ca[2];
            socket.emit("ban", JSON.stringify({"t": token, "pass": pass, "uid": uid}));
            break;
        case "/unban":
            var pass = ca[1];
            var uid = ca[2];
            socket.emit("unban", JSON.stringify({"t": token, "pass": pass, "uid": uid}));
            break;
        case "/dur":
            var dur = ca[1];
            cur_dur = dur;
            break;
        case "/join":
            var roomname = c.substring(6);
            socket.emit("join", JSON.stringify({"t": token, "roomname": roomname}));
            break;
        case "/switch":
            var roomname = c.substring(8);
            switchRoomByName(roomname);
            updateSidebar();
            break;
        case "/roomlist":
            socket.emit("roomlist",JSON.stringify({"t": token}));
            break;
        case "/onlineusers":
            socket.emit("onlineusers",JSON.stringify({"t": token}));
            break;
        default:
            $('#messages').append($('<li>').text("Unknown command"));
            break;
    }
    $('#m').val('');
}
