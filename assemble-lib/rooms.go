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
package assemble

import (
	"time"

	"github.com/Jeffail/gabs"
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
