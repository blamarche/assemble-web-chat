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

//var socket = io("", {reconnectionDelayMax:2000, reconnectionDelay:1000, timeout: 10000, multiplex:false });
var socket = null;
var rooms = {};
var roomnames = {};
var cur_room = "";
var cur_dur = "48h";
var token = window.location.hash.substring(1);
var newMsgCount=0;
var switchOnJoin = true;
var hasJoined = false;
var hasAuthed = false;
var hasHistory = false;
var enableSound = true;
var smallImages = false;
var noImages = false;
var firstTime = false;
var autoScroll = true;
var defaultHistory = 30;
var autoScrollMargin = 48;
var reconnectCount = 0;
var customicons = [];
var largeFont = false;
var useTheme = "";

var lc = null;

//load settings from local
function loadSettings() {
if (storageAvailable('localStorage')) {
    if (localStorage.getItem("useTheme"))
        useTheme = localStorage.getItem("useTheme");
    if (localStorage.getItem("enableSound"))
        enableSound = localStorage.getItem("enableSound") == "true";
    if (localStorage.getItem("autoScroll"))
        autoScroll = localStorage.getItem("autoScroll") == "true";
    if (localStorage.getItem("cur_dur"))
        cur_dur = localStorage.getItem("cur_dur");
    if (localStorage.getItem("smallImages"))
        smallImages = localStorage.getItem("smallImages") == "true";
    if (localStorage.getItem("noImages"))
        noImages = localStorage.getItem("noImages") == "true";
    if (localStorage.getItem("defaultHistory"))
        defaultHistory = parseInt(localStorage.getItem("defaultHistory"));
    if (localStorage.getItem("customicons"))
        customicons = JSON.parse(localStorage.getItem("customicons"));
    if (localStorage.getItem("largeFont"))
        largeFont = localStorage.getItem("largeFont")=="true";

    if (!localStorage.getItem("firstTime")) {
        localStorage.setItem("firstTime", "1");
        firstTime = true;
    }

    applyTheme();
} else {
    //console.log("No local storage");
}
}
loadSettings();

function serializeSettings() {
    var settings = {};
    for (var i=0; i<localStorage.length; i++) {
       settings[localStorage.key(i)] = localStorage.getItem(localStorage.key(i))
    }
    return JSON.stringify(settings);
}

function restoreSettings(settings) {
	try {
		settings = settings.trim();
		if (settings=="") return true;

		var settings = JSON.parse(settings);
		for (var prop in settings) {
			localStorage.setItem(prop, settings[prop]);
		}
		loadSettings();
		if (largeFont)
			$("body").addClass("largefont");
		else
			$("body").removeClass("largefont");
		return true;
	} catch (ex) {
		console.log("error loading settings");
		return false;
	}
}

//socket events
function hookSocketInit() {
socket.on('connect', auth);
socket.on('reconnect', auth);
function auth(d) {
    rooms={};
    roomnames={};
    updateSidebar();
    setTimeout(function(){
        setConnectMsg("Sending authentication token.");
	socket.emit("auth", token);
	setTimeout(function() {
            if (!hasAuthed)
                window.location.reload(true);
        }, 10000);
    }, 250);
}
socket.on('disconnect', function(d) {
    setConnectMsg("Reconnecting.");
    $(".connecting").removeClass("hidden");
    hasJoined=false;
    hasAuthed=false;
    reconnectCount++;
});

socket.io.on('connect_timeout', function(e) {
    //appendSystemMessage("socket connect timeout", 60000);
    console.log("socket timeout");
    console.log(e);
});
socket.io.on('connect_error', function(e) {
    //appendSystemMessage("socket connect error", 60000);
    console.log("socket error");
    console.log(e);
});
socket.io.on('reconnect_error', function(e) {
    //appendSystemMessage("socket reconnect error", 60000);
    console.log("socket reconnect error");
    console.log(e);
});

} //end hookSocketInit

hljs.initHighlightingOnLoad();


function applyTheme() {
    //remove all possible theme classes then add the current setting theme
    $("body").removeClass("dark-theme");
    $("body").removeClass("light-theme");
    if (useTheme!="") {
        $("body").addClass(useTheme);
    }
}

function setConnectMsg(msg) {
    if (document.getElementById("connectmsg")) {
	document.getElementById("connectmsg").innerHTML = msg;
    }
}

function editTheImage(ev) {
    /*
    if (ev.shiftKey) {
        var _im = ev.currentTarget;
        showLc(null);
        lc.saveShape(LC.createShape('Image', {x:0, y:0, image: _im}));          
        return false;
    } 
    */   
}

function showLc(e) {
    /*
    $('#literallycanvas').modal();
    if (!lc) {
        lc = LC.init(containerOne, {
        imageSize: {width:507, height:null},
        backgroundColor: 'rgba(255,255,255,1.0)'
        });
        ldz = new Dropzone("#literallyimgupFile",{
            url:"/",
            paramName: "file", // The name that will be used to transfer the file
            maxFilesize: 0.5, // MB
            previewsContainer: $("#preview")[0],
            clickable: true,
            maxFiles:1,
            acceptedFiles: "image/*",
            autoProcessQueue: false,
            thumbnail: function(file, imguri) {
            var newImage = new Image();
            newImage.src = imguri;
            lc.saveShape(LC.createShape('Image', {x:0, y:0, image: newImage}));
            }
        });
    }
	*/
}

$(document).ready(function(){
    if (largeFont) {
	$('body').addClass('largefont');
    }

    setConnectMsg("Establishing connection.");

    socket = io("", {reconnectionDelayMax:2000, reconnectionDelay:1000, timeout: 6000, multiplex:false });
    hookSocketInit();
    hookSocketChat();


    $( window ).resize(function() {
        updateSidebar();
    });

    // Adjust #messages margin-bottom when the height of the message textbox changes.
    // This prevents the textarea from covering lower messages.
    // Adapted from MoonLite: http://stackoverflow.com/a/16848663
    $('textarea#m').bind('mouseup mousemove',function(){
            if (this.oldheight === null) {
              this.oldheight = this.style.height;
            }
            if (this.style.height != this.oldheight) {
              $('#messages').css('margin-bottom', $(this).height()+20);
              $('html, body').scrollTop( $(document).height() - $(window).height() ); // Scroll to bottom
              this.oldheight = this.style.height;
            }
        });

    // If the window has scrolled to the bottom (or near bottom), hide the new message indicator
    $(window).scroll(function() {
      if($(window).scrollTop() + $(window).height() >= $(document).height()-autoScrollMargin) {
            $("#iconNewMsg").addClass("hidden");
      }
    });

    $ ( window ).on('keydown', function(ev) {
        if (ev.keyCode == 40 && ev.ctrlKey) { //down
            var troom = $("#sidebar li[data-room='"+cur_room+"']").next().attr("data-room");
            if (typeof troom!="undefined") {
                switchRoom(troom);
            }
            ev.preventDefault();
            return false;
        } else if (ev.keyCode == 38 && ev.ctrlKey) { //up
            var troom = $("#sidebar li[data-room='"+cur_room+"']").prev().attr("data-room");
            if (typeof troom!="undefined") {
                switchRoom(troom);
            }
            ev.preventDefault();
            return false;
        } else if (ev.keyCode == 72 && ev.ctrlKey) { //help (H)
            $('#firstTime').modal();
            ev.preventDefault();
            return false;
        }
    });

    //load in custom icons 
    for (var x=0; x<customicons.length; x++) {
        addCustomIcon(customicons[x]);
    }

    //push in smileys
    for (var x in icon_lib) { //its ok that these double. it'll only load once anyway
        var ic=$("<img>").attr("src","/icons/"+icon_lib[x]).attr("title", x);
        $("#iconPreload").append(ic);
        if (x.indexOf("(")==0) {
            $("#iconselect .modal-body").prepend(ic);
        }
    }

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

    //new message indicator
    $("#iconNewMsg").on('click',function() {
        scrollToBottom();
    });

    sendPing();
    function sendPing() {
        socket.emit("ping", JSON.stringify({"t": token}));
        setTimeout(sendPing, 149000);//~2.5 minutes. User timeouts every 5 minutes
        //TODO bandwidth optimize this! probably no need to send a full on token each time.
    }

    resetTitle();
    function resetTitle() {
        if (document.hasFocus() && newMsgCount>0) {
            document.title="Assemble Chat";
            newMsgCount=0;
        }
        setTimeout(resetTitle, 1500);
    }

    timeCalc();
    function timeCalc() {
        $("span.time").each(function(index, el){
            var d = new Date(parseInt($(el).attr('data-time')));
            var fuz = fuzzyTime(d);
            var t = $(el).html();
            if (t!=fuz) {
                $(el).html(fuz);
            }
        });
        setTimeout(timeCalc, 60000);
    }

    //alerts options
    $('#enablealerts').on('click', function(e) {
        socket.emit("setalerts", JSON.stringify({"t": token, "enabled": true}));
    });
    $('#disablealerts').on('click', function(e) {
        socket.emit("setalerts", JSON.stringify({"t": token, "enabled": false}));
    });

    //alerts options
    $('#darktheme').on('click', function(e) {
        useTheme="dark-theme";
        if (storageAvailable('localStorage'))
            localStorage.setItem("useTheme", useTheme);
        applyTheme();
    });
    $('#lighttheme').on('click', function(e) {
        useTheme="";
        if (storageAvailable('localStorage'))
            localStorage.setItem("useTheme", useTheme);
        applyTheme();
    });


    //scroll options
    $('#btnautoscrollon').on('click', function(e) {
        autoScroll = true;
        if (storageAvailable('localStorage'))
            localStorage.setItem("autoScroll", autoScroll);
    });
    $('#btnautoscrolloff').on('click', function(e) {
        autoScroll = false;
        if (storageAvailable('localStorage'))
            localStorage.setItem("autoScroll", autoScroll);
    });

    //sound options
    $('#enablesound').on('click', function(e) {
        enableSound = true;
        if (storageAvailable('localStorage'))
            localStorage.setItem("enableSound", enableSound);
    });
    $('#disablesound').on('click', function(e) {
        enableSound = false;
        if (storageAvailable('localStorage'))
            localStorage.setItem("enableSound", enableSound);
    });

    //font size options
    $("#normalfont").on('click', function(e) {
	$("body").removeClass("largefont");
	localStorage.setItem("largeFont", "");
	largeFont=false;
    });
    $("#largefont").on('click', function(e) {
        $("body").addClass("largefont");
	localStorage.setItem("largeFont", "true");
	largeFont=true;
    });

    //image size options
    $('#btnsmallimages').on('click', function(e) {
        noImages = false;
        smallImages=true;
        if (storageAvailable('localStorage')) {
            localStorage.setItem("smallImages", smallImages);
            localStorage.setItem("noImages", noImages);
        }
        $("#messages .messagetext img").addClass("smallimage");
        $("#messages .messagetext video").addClass("smallimage");
        $("#messages .messagetext iframe").addClass("smallimage");
    });
    $('#btnlargeimages').on('click', function(e) {
        noImages = false;
        smallImages=false;
        if (storageAvailable('localStorage')) {
            localStorage.setItem("smallImages", smallImages);
            localStorage.setItem("noImages", noImages);
        }
        $("#messages .messagetext img").removeClass("smallimage");
        $("#messages .messagetext video").removeClass("smallimage");
        $("#messages .messagetext iframe").removeClass("smallimage");
    });
    $('#btnnoimages').on('click', function(e) {
        noImages = true;
        if (storageAvailable('localStorage')) {
            localStorage.setItem("noImages", noImages);
        }
        $("#messages .messagetext img").hide();
        $("#messages .messagetext video").hide();
        $("#messages .messagetext iframe").hide();
    });

    //update profile options
    $('#btnupdateprofile').on('click', function(e) {
        $('#updateprofilebody').html('<iframe src="/signup/#token='+token+'"></iframe>');
        $('#updateprofile').modal();
    });

    //other buttons
    $("#messages").on('click', '.loadhistory', function(e) {
        reconnectCount = 0; //set this to 0 so the history is loaded at top of message log instead of the bottom
        var room = $(e.currentTarget).attr("data-room");
        $(e.currentTarget).parent().remove();
        socket.emit("history",JSON.stringify({"t": token, "room": room, "last": 0}));   //request history
    });

    $("#messages").on('click', '.userprofilelink', function(e) {
        socket.emit("userinfo", JSON.stringify({"t": token, "uid": $(e.currentTarget).attr("data-uid")}));
    });

    $("#inviteusertoroom").on('click', function(e) {
        var uid = $(e.currentTarget).attr("data-uid");
        socket.emit("inviteusertoroom", JSON.stringify({"t": token, "uid": uid, "room": cur_room}));
    });

    $("#btndeletemessage").on('click', function(e) {
        var msgid = $("#btndeletemessage").attr("data-msgid");
        requestDeleteMessage(msgid);
    });

    $("#btnleaveroom").on('click', function(e) {
        var rm = $("#btnleaveroom").attr("data-room");
        socket.emit("leave", JSON.stringify({"t": token, "room": rm}));
    });
    $("#btnhideroom").on('click', function(e) {
        var rm = $("#btnhideroom").attr("data-room");
        removeRoom(rm);
    });

    $("#createnewroom").on('click', function(e) {
        var name, isprivate, maxexptime, minexptime;
        name=$("#createroom .roomname").val();
        isprivate=$("#createroom .isprivate:checked").length;
        maxexptime=$("#createroom .maxexptime").val();
        minexptime=$("#createroom .minexptime").val();

        switchOnJoin=true;
        socket.emit("createroom", JSON.stringify({"t": token, "roomname": name, "maxexptime": maxexptime, "minexptime": minexptime, "isprivate": (isprivate!=0)}));

        $("#createroom .roomname").val("");
    });

    $("#togglesidebtn").on('click', function() {
      if ($('body').hasClass("sidebarhide")) {
        $('body').removeClass("sidebarhide");
      } else {
	$('body').addClass("sidebarhide");
      }
    });

    if (window.innerWidth < 500) {
	//hide bar on mobile/small windows by default
        $('body').addClass("sidebarhide");
    }

    $("#clearbtn").on('click', function() {
      $("#messages li[data-room='"+cur_room+"']").addClass("hidden");
    });

    //Paint panel with Literally Canvas - http://literallycanvas.com/
    //LC.setDefaultImageURLPrefix('/literallycanvas/img');
    //var lc = null;
    lc = null;
    var ldz = null;
    containerOne = document.getElementsByClassName('literally one')[0];
    $('#literallycanvasbtn').on('click', showLc);
    
    $('#literallycanvas').on('shown.bs.modal', function (e) {
    // This prevents a 0x0 canvas when the window is resized and the modal is hidden.
    window.dispatchEvent(new Event('resize'));
    });

    //disable the normal dialog from showing
    $("#literallyimgupFile").on('click',function() {
        //return false;
    });
    $("#literallyimgup").on('click',function() {
        $("#literallyimgupFile").click();
    });
    $('#literallypost').on('click', function(e) {
      e.preventDefault();
      var pngString = lc.getImage().toDataURL("image/jpeg",0.9);
      socket.emit('chatm', JSON.stringify({"t": token, "room": cur_room, "m": pngString, "dur":cur_dur}));
      $('#literallycanvas').modal('hide');
    });

    $("#sendmessage").on('click', function(e) {
        var uid = $(e.currentTarget).attr("data-uid");
        switchOnJoin=true;
        socket.emit("directmessage",JSON.stringify({"t":token, "uid": uid}));
    });

    $('#messages').on('click', 'a.joinroom', function(ev){
        var rm = $(ev.currentTarget).attr("data-room");
        socket.emit("join", JSON.stringify({"t": token, "roomid": rm}));
        if (typeof rooms[rm]!="undefined") {
            switchRoom(rm);
            updateSidebar();
        } else {
            switchOnJoin=true;
        }
        return false;
    });

    //shrink/enlarge images
    $('#messages').on('click', '.messagetext img', imgtoggle);
    //$('#messages').on('contextmenu', '.messagetext img', imgtoggle);
    $('#messages').on('click', '.messagetext iframe', imgtoggle);
    //$('#messages').on('contextmenu', '.messagetext iframe', imgtoggle);
    //$('#messages').on('contextmenu', '.messagetext video', imgtoggle); //firefox bug causes issues clicking on a control
    $('#messages').on('click', '.messagetext video', imgtoggle); //firefox bug causes issues clicking on a control
    function imgtoggle(ev) {
        if (ev.shiftKey) {
            return true;
        }

        if (!$(ev.currentTarget).hasClass("smiley") && !$(ev.currentTarget).hasClass("avatar")) {
            if ($(ev.currentTarget).hasClass("smallimage")) {
                $(ev.currentTarget).removeClass("smallimage");
            } else {
                $(ev.currentTarget).addClass("smallimage");
            }
        }
        return false;
    }

    $("#customiconurl").on('drop', addiconevent);
    $("#customiconurl").on('paste', addiconevent);
    function addiconevent(ev) {
        if (ev.originalEvent.clipboardData) { 
            ev.originalEvent.dataTransfer = ev.originalEvent.clipboardData;
        }
        var icurl =  ev.originalEvent.dataTransfer.getData('text');//ev.currentTarget.value;
        if (icurl!="") {
            addCustomIcon(icurl);
            customicons.push(icurl);
            localStorage.setItem("customicons", JSON.stringify(customicons));
            ev.preventDefault();
            $("#customiconurl").val("");
            return false;
        }
    }

    $("#iconselect").on('contextmenu', '.modal-body img', function(ev){
        if ($(ev.currentTarget).hasClass("customiconselect")) {
            var icurl = $(ev.currentTarget).attr("src");
            for (var x=0; x<customicons.length; x++) {
                if (customicons[x]==icurl) {
                    customicons.splice(x,1);
                    localStorage.setItem("customicons",JSON.stringify(customicons));
                }
            }
            $(ev.currentTarget).remove();
        }
        ev.preventDefault();
        return false;
    });

    $("#iconselect").on('click', '.modal-body img', function(ev){
        var ic=$(ev.currentTarget).attr("title");
        $("#iconselect").modal('hide');
        if ($("#m").val()=="") {
            $("#m").val(ic);
            $("form").submit();
        } else {
            $("#m").val($("#m").val()+" "+ic);
        }
	focusInput();
        ev.preventDefault();
        return false;
    });

    $('#options .currentduration').change(function(e) {
        cur_dur = $('#options .currentduration').val();
        if (storageAvailable('localStorage'))
            localStorage.setItem("cur_dur", cur_dur);
    });

    $('#options .defaulthistory').change(function(e) {
        defaultHistory = parseInt($('#options .defaulthistory').val());
        if (storageAvailable('localStorage'))
            localStorage.setItem("defaultHistory", defaultHistory);
    });

    if (firstTime) {
        $('#firstTime').modal();
    }

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
var makeline = true;
$("#m").on('keydown', function(ev) {
    if (ev.keyCode==13) {
        if (ev.shiftKey) {
            if(makeline){
                $("#m").val($("#m").val()+"\n");
                makeline = false;
            }
        } else {
            $('form').submit();
            ev.preventDefault();
            return false;
        }
    }
});

$("#m").on('keyup',function(ev){
    if (ev.keyCode == 13){
        makeLine=true;
    }
});
//menu buttons.
$("#btnroomlist").on('click', function() {
    socket.emit("roomlist",JSON.stringify({"t": token}));
});
$("#btncreateroom").on('click', function() {
    $("#createroom").modal();
});
/*$("#btnduration").on('click', function() {
    $("#messageduration").modal();
});*/

$("#backupsettings").on('click', function() {
	var s = serializeSettings();
	appendSystemMessage(s, 60000);
	alert("You will find your settings string in the chat window. Please save this string in a text file in case you need to restore later.");
});
$("#restoresettings").on('click', function() {
	restoreSettings(prompt('Paste your settings string here:','{}'));
	alert("Settings have been restored. You will need to refresh the page to see your custom icons.");
});

$("#btnoptions").on('click', function() {
    $("#options").modal();
});
$("#btninvitenewuser").on('click', function() {
    var emm=prompt("Enter a message for the invited user (optional):");
    if (typeof emm!="undefined")
        socket.emit("invitenewuser", JSON.stringify({"t": token, "email": emm}));
});
$("#btnuserlist").on('click', function() {
    socket.emit("onlineusers",JSON.stringify({"t": token}));
});
$("#btnroomusers").on('click', function() {
    socket.emit("roomusers",JSON.stringify({"t": token, "room": cur_room}));
});

function hookSocketChat() {
socket.on('chatm', function(d){
    d=JSON.parse(d);
    appendChatMessage(d.uid,d.room,d.name,d.nick,d.m,d.msgid,d.avatar,d.time);

    if ($("#m").prop('disabled')==true) {
        $("#m").prop('disabled', false);
        focusInput();
    }
    $("#m").prop('disabled', false);

    if (!document.hasFocus() && enableSound) {
        $("#sfxbeep")[0].play();
    }
    if (!document.hasFocus()) {
        newMsgCount++;
        document.title = "("+newMsgCount+") "+rooms[d.room].friendlyname;
    }

    try {
        if (Notification.permission==="granted" && (cur_room!=d.room || !document.hasFocus()) && d.m.indexOf("data:image/")!=0) {
            var notification = new Notification(d.nick+": "+($("<div/>").html(d.m).text().substring(0,256))+" ["+d.name+"]");
            setTimeout(function() {
                notification.close();
            },7000);
        }
    }catch (err) {
        console.log(err);
    }

    if (cur_room!=d.room) {
        rooms[d.room].mcount++;
        updateSidebar();
    }
});

socket.on('setalerts', function(msg) {
    appendSystemMessage(msg, 30000)
});

socket.on('leave', function(room) {
    removeRoom(room);
});

function removeRoom(room) {
    delete rooms[room];
    for (var n in roomnames) {
        if (roomnames[n]==room) {
            delete roomnames[n]
            break;
        }
    }
    $("#sidebar li[data-room='"+room+"']").remove();
}

socket.on('inviteusertoroom', function(d) {
    var d=JSON.parse(d);
    var m = "<span class='prefix'>You've been invited to join </span>";
    m+="<a class='joinroom' data-room='"+d.room+"'>"+d.name+"</a>";
    m+="<div class='clearfloat'></div>";
    appendSystemMessage(m, 0);
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

socket.on('roomusers', function(d) {
    var d=JSON.parse(d);
    var m = "<span class='prefix'>Users in this room: </span>";
    for (var i=0; i<d.uids.length; i++) {
        if (d.online[i])
            m+="<a class='userprofilelink onlineuser' data-uid='"+d.uids[i]+"'>"+d.nicks[i]+"</a>";
        else
            m+="<a class='userprofilelink onlineuser offline' data-uid='"+d.uids[i]+"'>"+d.nicks[i]+"</a>";
    }
    m+="<div class='clearfloat'></div>";
    $('#messages li.userlist').slideUp(500);
    appendSystemMessage(m,15000,'userlist');
});

socket.on('onlineusers', function(d) {
    var d=JSON.parse(d);
    var m = "<span class='prefix'>Total Online Users: </span>";
    for (var i=0; i<d.uids.length; i++) {
        m+="<a class='userprofilelink onlineuser' data-uid='"+d.uids[i]+"'>"+d.nicks[i]+"</a>";
    }
    m+="<div class='clearfloat'></div>";
    $('#messages li.userlist').slideUp(500);
    appendSystemMessage(m,15000,'userlist');
});

socket.on('roomlist', function(d){
    var d=JSON.parse(d);
    var m = "<span class='prefix'>Room List: </span>";
    for (var k in d) {
        m+="<a class='joinroom' data-room='"+k+"'>"+d[k]+"</a>";
    }
    m+="<div class='clearfloat'></div>";
    $('#messages li.roomlist').slideUp(500);
    appendSystemMessage(m, 15000, 'roomlist');
});

socket.on('history', function(d){
    setJoined();

    var d=JSON.parse(d);
    var added = 0;
    if (reconnectCount==0) {
        for (var i = d.history.length - 1; i >= 0; i--) {
            added += appendChatMessage(d.history[i].uid,d.room,d.name,d.history[i].nick,d.history[i].m,d.history[i].msgid, d.history[i].avatar,d.history[i].time, 'prepend');
        }
    } else {
        for (var i = 0; i < d.history.length; i++) {
            added += appendChatMessage(d.history[i].uid,d.room,d.name,d.history[i].nick,d.history[i].m,d.history[i].msgid, d.history[i].avatar,d.history[i].time);
        }
    }
    updateSidebar();

    var li = $("#messages li.chatmsg a.loadhistory.tmp[data-room='"+d.room+"']").parent().remove();

    if (added>0) {
        //bug fix applying .chatmsg to load more links
        var hiddenclass="";
        if (d.room!=cur_room)
            hiddenclass="hidden";
        appendSystemMessage("<a class='loadhistory' data-room='"+d.room+"'>Load history...</a>",0, "chatmsg", 'prepend').removeClass("sysmsg").addClass(hiddenclass).attr('data-room', d.room);
    }
});

var histRetry = null;
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
        hasJoined=true;
	rooms[d.room] = {users: [], messages: [], friendlyname: d.name, mcount: 0, minexptime: d.minexptime, maxexptime: d.maxexptime};
        roomnames[d.name] = d.room;
        if (d.room=="lobby" || switchOnJoin) {
            switchRoom(d.room);
            switchOnJoin=false;
        }
        var t = (new Date()).getTime()/1000;
        //appendChatMessage("", d.room, d.name, "<em>SYSTEM</em>", "Joined "+d.name+" ("+d.minexptime+" - "+d.maxexptime+")", "", "/icons/icon_important.svg", t, 'prepend');
	var hiddenclass="";
        if (d.room!=cur_room)
            hiddenclass="hidden";
  	appendSystemMessage("<a class='loadhistory tmp' data-room='"+d.room+"'>Load history...</a>",0, "chatmsg", 'prepend').removeClass("sysmsg").addClass(hiddenclass).attr('data-room', d.room);

	setTimeout(function() {
	    socket.emit("history",JSON.stringify({"t": token, "room": d.room, "last": defaultHistory}));   //request history
	}, 750);

	setConnectMsg("Room joined, requesting message history.");

	setTimeout(function() {
	    setConnectMsg("Room joined, requesting message history.<br>Reticulating splines, retrying in 5 seconds.");
	}, 5000);
	clearTimeout(histRetry);
	histRetry = setTimeout(retryHistoryLoop, 10000);
    } else if (hasJoined) {
        setJoined();
    }
    updateSidebar();
});

function retryHistoryLoop() {
    if (!hasHistory) {
         for (var rt in rooms) {
              socket.emit("history",JSON.stringify({"t": token, "room": rooms[rt], "last": defaultHistory}));
         }
         setConnectMsg("Retrying message history request.");
	 setTimeout(function() {
             setConnectMsg("Requesting message history.<br>Retrying again in 5 seconds.");
         }, 5000);
	 histRetry = setTimeout(retryHistoryLoop, 10000);
    }
}

socket.on('joined', function(d){
    var d=JSON.parse(d);
    //appendSystemMessage(d.nick +" joined "+d.name, 3000);
    rooms[d.room].users.push({uid:d.uid, nick:d.nick});
});

socket.on('auth_error', function(d){
    appendSystemMessage("Error: "+d,5000);
    if (d=="Invalid Token") {
        appendSystemMessage("<a class='signup' href='/signup'>Sign up with your Invite Code</a>",0);
        $(".connecting").addClass("hidden");
    }
    $("#m").prop('disabled', false);
});

socket.on('auth', function(d){
    //appendSystemMessage("Logged in successfully",10000);
    hasAuthed=true;
    setConnectMsg("Authenticated. Waiting for room data.");
    setTimeout(function() {
        if (!hasJoined)
            window.location.reload(true);
    }, 7500);
});

socket.on('invitenewuser', function(d){
    var d=JSON.parse(d);
    appendSystemMessage("Invite Key: "+d.key+" <a href='/signup/#"+d.key+"'>(signup link)</a>",0);
});

socket.on('deletechatm', function(d){
    if ($("#messages li.chatmsg .messagetext[data-msgid='"+d+"']").parent().children(".messagetext").length <= 1) {
        var li = $("#messages li.chatmsg .messagetext[data-msgid='"+d+"']").parent().parent().parent(); //yikes!
        
        li.html("<i style='display:block;'>Removed message</i>");
        setTimeout(function(){
            li.slideUp(1000, function() {
                li.remove();
            });
        }, 3000);
    } else {
        var li=$("#messages li.chatmsg .messagetext[data-msgid='"+d+"']");
        li.html("<i>Removed message</i>");
        li.removeClass("messagetext");
        li.addClass("deletedmessage");
        setTimeout(function(){
            li.slideUp(1000, function() {
                li.remove();            
            });
        }, 3000);
    }
});

}// end hookSocketChat

function setJoined() {
    if (!$(".connecting").hasClass("hidden"))
	$(".connecting").addClass("hidden");

    focusInput();
    hasJoined=true;
    hasHistory=true;
}

function updateSidebar() {
    //update sidebar to hold message counts and list active chat rooms
    for (var r in rooms) {
        var rm = rooms[r];
        if ($("#sidebar li[data-room='"+r+"']").length == 0) {
            $("#sidebar").append($("<li>")
                .attr("data-room", r)
                .attr("title", rm.minexptime+" - "+rm.maxexptime)
                .html("<div class='title'>"+rm.friendlyname+"</div><span></span>")
                .on('click', function(ev) {
                    var rm = $(ev.currentTarget).attr("data-room");
                    switchRoom(rm);
                })
                .on('contextmenu', function(ev) {
                    var rm = $(ev.currentTarget).attr("data-room");
                    //socket.emit("leave", JSON.stringify({"t": token, "room": rm}));
                    $("#btnleaveroom").attr("data-room", rm);
                    $("#btnhideroom").attr("data-room", rm);
                    $("#leaveroommodal").modal();
                    return false;
                })
            );
        }

        if (rm.mcount > 0) {
            $("#sidebar li[data-room='"+r+"'] span").html(rm.mcount);
            $("#sidebar li[data-room='"+r+"']").addClass("newmsg");
        } else {
            $("#sidebar li[data-room='"+r+"'] span").text("");
            $("#sidebar li[data-room='"+r+"']").removeClass("newmsg");
        }
        if (r == cur_room) {
            $("#sidebar li.active").removeClass("active");
            $("#sidebar li[data-room='"+r+"']").addClass("active");
        }
    }
    $("#sidebar").css("height", (window.innerHeight-42)+"px");
}

function scrollToBottom() {
    if (autoScroll)
        $(window).scrollTop($('body')[0].scrollHeight);
}

function appendSystemMessage(msg, lifetimeMs, cssclass, mode) {
    if (typeof cssclass=="undefined")
        cssclass="";
    if (typeof mode=="undefined")
        mode="append"; //or prepend

    var sm = $('<li>').addClass("sysmsg").addClass(cssclass).html(msg);

    // If scrolled to bottom already, scroll to bottom again after appending message
    var atBottom = false;
    if ($(window).scrollTop() + $(window).height() >= $(document).height()-autoScrollMargin) {
      atBottom = true;
    }
    if (mode=="append")
        $('#messages').append(sm);
    if (mode=='prepend')
        $('#messages').prepend(sm);
    if (lifetimeMs>0) {
        setTimeout(function(){
            sm.slideUp(1000);
        }, lifetimeMs);
    }
    if (atBottom) {
      scrollToBottom();
    } else {
      $('#iconNewMsg').removeClass("hidden");
    }

    return sm;
}

/**
 * Get the value of a querystring - useful for embedded youtube start time
 * Source: http://gomakethings.com/how-to-get-the-value-of-a-querystring-with-native-javascript/
 * @param  {String} field The field to get the value of
 * @param  {String} url   The URL to get the value from (optional)
 * @return {String}       The field value
 */
var getQueryString = function ( field, url ) {
    var href = url ? url : window.location.href;
    var reg = new RegExp( '[?&]' + field + '=([^&#]*)', 'i' );
    var string = reg.exec(href);
    return string ? string[1] : null;
};

function appendChatMessage(uid, room, roomname, nick, m, id, avatar, time, mode) {
    if (typeof mode=="undefined") {
        mode="append"; //other option: prefix
    }
    if (id!="") {
        var msgelem = $("#messages li.chatmsg[data-msgid='"+id+"']");
        var msgelem2 = $("#messages li.chatmsg .messagetext[data-msgid='"+id+"']");
        if (msgelem.length > 0 || msgelem2.length > 0)
            return 0;
    }

    var hide = "";
    if (room!=cur_room) {
        hide="hidden";
    }

    var tmp = $('<div>')
    tmp.text(m);
    m=tmp.text();

    var small = smallImages ? " smallimage" : "";

    if (m.indexOf("data:image/")==0) {
        if (noImages)
            m = "<a class='autolink upload' target='_blank' href='"+m+"'>Image Uploaded</a>";
        else
            m = "<img class='autolink upload"+small+"' src='"+m+"'></img>";
    } else {
	    var iscode = m.indexOf("!code ")==0;
        m = Autolinker.link(m, {
            stripPrefix: false,
            truncate: 30,
            className:"autolink",
            twitter:false,
            hashtag: false,
            replaceFn : function( autolinker, match ) {
                href = match.getAnchorHref();
                switch( match.getType() ) {
                    //TODO cleanup!
                    case 'url' :
                        if ( match.getUrl().indexOf( '.jpg' ) !== -1 ||
                             match.getUrl().indexOf( '.jpeg' ) !== -1 ||
                             match.getUrl().indexOf( '.png' ) !== -1 ||
                             match.getUrl().indexOf( '.gif' ) !== -1  )
                        {
                            if (noImages)
                                return "<a href='"+href+"' target='_blank'>"+href+"</a>";
				
			    if (match.getUrl().indexOf( '#iconsmall' ) !== -1) {
				return "<img src='"+href+"' class='smiley gif'></img>";
			    }
			    if (match.getUrl().indexOf( '#icon' ) !== -1) {
                                return "<img src='"+href+"' class='smiley large gif'></img>";
                            }

                            return "<a href='"+href+"' target='_blank'>"+href+"</a><br><img src='"+href+"' class='autolink"+small+"'></img>";
                        }
                        else if ( match.getUrl().indexOf( '.mp4' ) !== -1 ||
                             match.getUrl().indexOf( '.ogg' ) !== -1 ||
                             match.getUrl().indexOf( '.webm' ) !== -1 )
                        {
                            if (noImages)
                                return "<a href='"+href+"' target='_blank'>"+href+"</a>";

                            return "<a href='"+href+"' target='_blank'>"+href+"</a><br><video controls class='autolink"+small+"'+small+''><source src='"+href+"'></video>";
                        }
                        else if ( match.getUrl().indexOf('youtube.com/watch?v=') !== -1 )
                        {
                            if (noImages)
                              return "<a href='"+href+"' target='_blank'>"+href+"</a>";

                            var frame = "<a href='"+href+"' target='_blank'>"+href+"</a><br>"+'<iframe class="autolink'+small+'" height="315" src="'+match.getUrl().replace('youtube.com/watch?v=', 'youtube.com/embed/')+'" frameborder="0" allowfullscreen></iframe>';

                            // Convert youtube url if it contains a query string for start time so embedding works
                            var embedTime = getQueryString('t', href);
                            if (embedTime) {
                              var youtubeV = getQueryString('v', href);
                              var strmins = embedTime.slice(0, embedTime.indexOf("m"));
                              var youtubeMinutes = parseInt(strmins,10);
                              if (embedTime.indexOf("m") !== -1) {
                                // time includes minutes, split string to work on seconds
                                embedTime = embedTime.split('m')[1];
                              } else {
                                youtubeMinutes = 0;
                              }
                              var strsecs = embedTime.slice(0, embedTime.indexOf("s"));
                              var youtubeSeconds = parseInt(strsecs, 10);
                              var youtubeT = youtubeMinutes*60 + youtubeSeconds;

                              frame = "<a href='"+href+"' target='_blank'>"+href+"</a><br>"+'<iframe class="autolink'+small+'" height="315" src="//www.youtube.com/embed/'+youtubeV+'?start='+youtubeT+'"frameborder="0" allowfullscreen></iframe>';
                            }
                            return frame;
                        }
                        else if ( match.getUrl().indexOf('youtu.be/') !== -1 )
                        {
                            if (noImages)
                                return "<a href='"+href+"' target='_blank'>"+href+"</a>";

                            var frame = "<a href='"+href+"' target='_blank'>"+href+"</a><br>"+'<iframe class="autolink'+small+'" height="315" src="'+match.getUrl().replace('youtu.be/', 'youtube.com/embed/')+'" frameborder="0" allowfullscreen></iframe>';

                            // Convert youtube url if it contains a query string for start time so embedding works
                            var embedTime = getQueryString('t', href);
                            if (embedTime) {
                              // Get the embed code between '/' and '?
                              var youtubeV = href.split('?')[0];
                              youtubeV = youtubeV.split('/');
                              youtubeV = youtubeV.slice(-1)[0]
                              var strmins = embedTime.slice(0, embedTime.indexOf("m"));
                              var youtubeMinutes = parseInt(strmins,10);
                              if (embedTime.indexOf("m") !== -1) {
                                // time includes minutes, split string to work on seconds
                                embedTime = embedTime.split('m')[1];
                              } else {
                                youtubeMinutes = 0;
                              }
                              var strsecs = embedTime.slice(0, embedTime.indexOf("s"));
                              var youtubeSeconds = parseInt(strsecs, 10);
                              var youtubeT = youtubeMinutes*60 + youtubeSeconds;

                              frame = "<a href='"+href+"' target='_blank'>"+href+"</a><br>"+'<iframe class="autolink'+small+'" height="315" src="//www.youtube.com/embed/'+youtubeV+'?start='+youtubeT+'"frameborder="0" allowfullscreen></iframe>';
                            }
                            return frame;
                        }
                        break;
                }
                return;
            }
        });

        m = m.split("\n").join("<br>");

        //icons
	if (!iscode)
        	m = processIcons(m);
	else {
		m = '<pre><code>'+m.substring(6)+'</code></pre>';
	}
    }

    if (avatar=="" || typeof avatar=="undefined") {
        avatar = "";
    }

    var rawtime=time*1000;
    if (typeof time=="undefined") {
        time="";
    } else {
        time = fuzzyTime(new Date(time*1000));
    }

    var avatarimg = "<img src='"+avatar+"' class='avatar'></img>";
    if (avatar=="") {
        avatarimg = "<span class='glyphicon glyphicon-user avatar'></span>";
    }

    var lastmsgli = $("#messages li.chatmsg[data-room='"+room+"']").last();
    if (mode=="prepend")
        lastmsgli = $("#messages li.chatmsg[data-room='"+room+"']").first();
 
    var lastuid = lastmsgli.find(".messagecontainer a.nick").attr("data-uid");
    var msgli = $('<li>');

    var lasttime = lastmsgli.find(".time");
    var rawlasttime = parseInt(lasttime.attr("data-time"));
    var collapse = false;
    if (Math.abs(rawtime - rawlasttime) <= 2*60*1000) {
        collapse=true;
    }

    if (lastuid==uid && collapse) {
        msgli = $("<span>")
            .html(m)
            .addClass("messagetext")
            .addClass("collapsed")
            .attr("data-msgid", id);

        if (mode=="append") {
            lasttime.attr("data-time", rawtime);
            lasttime.html(time);
        }

        msgli.on("contextmenu", delfunc);
        msgli.find('img.upload').on('click', editTheImage);
    } else {
        msgli = $('<li>')
            .html("<div class='useravatar'>"+avatarimg+"</div><div class='messagecontainer'><a title='"+uid+"' data-uid='"+uid+"' class='userprofilelink nick'>"+nick+"</a> <span class='time' data-time='"+rawtime+"'>"+time+"</span> <br><span class='messagesubcontainer'><span class='messagetext'>"+m+"</span></span>")
            .attr("data-msgid", id)
            .attr("data-room", room)
            .addClass("chatmsg")
            .addClass(hide);

        msgli.find('.messagetext').attr("data-msgid", id);
        
        msgli.children('.useravatar').on("contextmenu", delfunc);
        msgli.find('a').on("contextmenu", delfunc);
        msgli.find('.messagetext').on("contextmenu", delfunc);

        msgli.find('.messagetext img.upload').on('click', editTheImage);
    }

    function delfunc(ev) {
        //console.log(ev.target);
        if (!$(ev.target).hasClass("messagetext"))
            return true;

        var msgid = $(ev.currentTarget).attr("data-msgid");
        if (typeof msgid=="undefined") {
            msgid = $(ev.currentTarget).parent().attr("data-msgid");
            if (typeof msgid=="undefined")
                msgid = $(ev.currentTarget).parent().parent().attr("data-msgid");
        }
        $("#btndeletemessage").attr("data-msgid", msgid);
        $("#deletemessagemodal").modal();
        return false;
    }

    // If scrolled to bottom already (or nearly), scroll to bottom again after appending message
    var atBottom = false;
    if($(window).scrollTop() + $(window).height() >= $(document).height()-autoScrollMargin) {
        atBottom = true;
    }

    if (mode=="append") {
        if (uid==lastuid && collapse) {
            lastmsgli.find(".messagesubcontainer").append(msgli);
        }
        else
            $('#messages').append(msgli);
    }
    else if (mode=="prepend") {
        if (uid==lastuid && collapse) {
            lastmsgli.find(".messagesubcontainer").prepend(msgli);            
        }
        else    
            $('#messages').prepend(msgli);
    }

    msgli.find('pre code').each(function(i, block) {
    	hljs.highlightBlock(block);
    });

    if (atBottom) {
        var imgs = msgli.find('.messagetext img');
        if (imgs.length>0) {
            imgs.first().load(function() {
                scrollToBottom();
            });
        } else {
            var iframes = msgli.find('.messagetext iframe');
            var videos = msgli.find('.messagetext video');
            if (iframes.length>0) {
                setTimeout(function() {
                    scrollToBottom();
                }, 1000);
            } else if (videos.length) {
                videos.first().on('loadedmetadata', function() {
                    scrollToBottom();
                });
            }
        }
        scrollToBottom();
    } else {
        $('#iconNewMsg').removeClass("hidden");
    }

    return 1;
}

function processIcons(m) {
    for (var x in icon_lib) {
        if (m==x){
            m='<img src="/icons/'+icon_lib[x]+'" class="smiley large" />';
        }
        m=m.split(x).join('<img src="/icons/'+icon_lib[x]+'" class="smiley" />');
        //needs to be smarter about where it does replacements?
    }
    m = asciimoji(m);
    return m;
}

function requestDeleteMessage(msgid) {
    socket.emit("deletechatm", JSON.stringify({
        "t": token,
        "room": cur_room,
        "msgid": msgid
    }));
}

function switchRoom(room) {
    if (room!="lobby") {
        $("#messages li.sysmsg").hide();
    }

    if (typeof rooms[room]=="undefined") {
        appendSystemMessage("Unknown room", 5000);
    } else {
        cur_room = room;
    }

    $("#messages li.chatmsg").addClass("hidden");
    $("#messages li.chatmsg[data-room='"+cur_room+"']").removeClass("hidden");

    rooms[cur_room].mcount = 0;

    updateSidebar();
    scrollToBottom();
    focusInput();
}

function focusInput() {
    if (!isIOSDevice())
        $("#m").focus();
    else
	$("#m").blur();
}

function isIOSDevice() {
    return navigator.userAgent.match(/iPhone/i) || navigator.userAgent.match(/iPad/i);
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
        case "/?":
            $('#firstTime').modal();
            break;
        case "/help":
            appendSystemMessage(" \
                /leave - Leaves the current room <br>\
                /ban admin uid - Bans the UID permanently <br>\
                /unban admin uid - UnBans the UID <br>\
                /dur message-duration - Sets your message expiration time (ie: 24h, 10m, 30s, etc) <br>\
                /join room-name - Attempts to join a room by name <br>\
                /switch room-name - Switches your chat focus to a room by name <br>\
                /roomlist - Lists all public rooms with links to join <br>\
            ", 15000);
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
            appendSystemMessage("Unknown command", 3000);
            break;
    }
    $('#m').val('');
}

function addCustomIcon(x) {
    var ic=$("<img>").attr("src",x).attr("title", x+"#icon").addClass("customiconselect");
    $("#customicons").append(ic);
}

function storageAvailable(type) {
	try {
		var storage = window[type],
			x = '__storage_test__';
		storage.setItem(x, x);
		storage.removeItem(x);
		return true;
	}
	catch(e) {
		return false;
	}
}

var icon_lib = {
    ">:|":"angry.svg",
    ">:(":"angry.svg",
    ":D":"bigsmile.svg",
    ":-D":"bigsmile.svg",
    ":$":"blush.svg",
    ":-$":"blush.svg",
    "o.O":"confused.svg",
    "O.o":"confused.svg",
    "O_o":"confused.svg",
    "o_O":"confused.svg",
    "8-)":"shades.svg",
    ";(":"cry.svg",
    ":'(":"cry.svg",
    ";-(":"cry.svg",
    "(important)":"important.svg",
    ":*":"kiss.svg",
    "X-D":"lol.svg",
    ":|":"neutral.svg",
    ":-|":"neutral.svg",
    ":(":"sad.svg",
    ":-|":"neutral.svg",
    ":-(":"sad.svg",
    ":-#":"sick.svg",
    ":)":"smile.svg",
    ":-)":"smile.svg",
    ":O":"surprised.svg",
    ":-O":"surprised.svg",
    "(thinking)":"think.svg",
    ":P":"tongue.svg",
    ":-P":"tongue.svg",
    "(twisted)":"twisted.svg",
    ";)":"wink.svg",
    ";-)":"wink.svg",

    "(angry)":"angry.svg",
    "(bigsmile)":"bigsmile.svg",
    "(blush)":"blush.svg",
    "(confused)":"confused.svg",
    "(shades)":"shades.svg",
    "(cry)":"cry.svg",
    "(kiss)":"kiss.svg",
    "(lol)":"lol.svg",
    "(neutral)":"neutral.svg",
    "(sad)":"sad.svg",
    "(sick)":"sick.svg",
    "(smile)":"smile.svg",
    "(surprised)":"surprised.svg",
    "(tongue)":"tongue.svg",
    "(wink)":"wink.svg",
    "(eek)":"eek.svg",
    "(y)":"y.svg",
    "(n)":"n.svg",
    "(facepalm2)":"facepalm2.jpg",
    "(facepalm)":"facepalm.svg"
};
