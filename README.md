# assemble-chat-server
Assemble is a GPLv3, secure html-based chat system meant to be self-hosted among a group of friends. It supports auto-deletion of messages, encryption, and minimal drive-based storage of data.

## Features Implemented
* Golang based Https and Socket server
* User token generation/signup (pub/priv segments) and sign-in.
* New user invite process
* Ban/unban system
* In-memory only storage system for chat rooms, history, etc
* Manual message deletion
* User-configurable per-message auto-delete time
* Create/Join Chat Rooms
* List public chat rooms
* Phone friendly, Tablet friendly UI
* Auto-process message content for links, image embeds, etc

## Priority Features To Be Implemented
* Invite to chat rooms
* List of users in a room & online status
* User timeouts/disconnects
* Private chat rooms with invite/moderation process
* Basic "emoticons"
* User token 'sharing' to other user-owned devices once signed in
* More...

## Stretch Features to Consider
* Direct Messaging
* (Optional) Text message / push notification process
* (Optional) Inter-server communication system
* Image uploads in messages
* Client-side addition of "emoticons/stickers"

### User Token structure
UID, PRIVID, nick, name, email, phone, url, desc, avatar

### Message structure
MSGID, UID, room, expTime, contents, sentOn,

### Room structure
privOrPub, name, creatorUID, membersUIDs, invitedUIDs, maxExpTime, minExpTime, avatar, maxHistory

#### Potential Gotchas
Full token must be sent with each message to verify authenticity
