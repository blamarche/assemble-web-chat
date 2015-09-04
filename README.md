# assemble-chat-server
Assemble is a GPLv3, database-less, full featured html chat system meant to be quick to set up and easy to host. It supports auto-expiration of messages, autoparses links and images and more.

## Features Implemented
* Golang based Https and Socket server
* User token generation/signup (pub/priv segments) and sign-in.
* New user invite process
* Ban/unban system
* In-memory only storage system for chat rooms, history, etc
* Manual message deletion
* User-configurable per-message expiration time
* Create/Join Chat Rooms
* Phone friendly, Tablet friendly UI
* Auto-process message content for links, image embeds, etc
* User avatars
* Image uploads in messages
* Desktop notifications API
* List of users & online status
* Invite to chat rooms
* Private chat rooms with invite
* Direct Messaging
* Text message / email notifications
* Smilies
* Auto-embed video links
* Avatar generator

## Yet To Be Implemented
* Direct messages to offline users
* Thumbs up / down smiley
* Validation / error proofing
* Update page title when new messages and not focused. When focused, reset back
* Client-side addition of custom "emoticons/stickers"
* User token 'sharing' to other user-owned devices once signed in
* Moderation process (ie /kick for the creator)
* "@user" highlights in messages
* Confirm leave room dialog

## Other Features to Consider
* Optional token pass-phrase set on sign-up
* VOIP
* In-memory encryption of messages
* Inter-server communication system

### Special Thanks
Thanks goes to Sebastian Kraft for providing public domain smilies: http://opengameart.org/content/cubikopp-qubic-smilies
