# assemble-chat-server
Assemble is a GPLv3, database-less, full featured html chat system meant to be quick to set up and easy to host. It supports auto-expiration of messages, autoparses links and images and more.

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
* User avatars
* Image uploads in messages
* Unread / new messages
* Desktop notifications API
* List of users in a room & online status
* Invite to chat rooms

## Features Yet To Be Implemented
* Ui dialogs for create-room/roomlist/invite/msg-duration
* Show chat room default message expirations on-join
* Room avatars
* Private chat rooms with invite/moderation process (ie /kick for the creator)
* Basic "emoticons"
* User token 'sharing' to other user-owned devices once signed in
* More...

## Other Features to Consider
* Direct Messaging
* (Optional) Text message / push notification process
* (Optional) Inter-server communication system
* Client-side addition of "emoticons/stickers"
