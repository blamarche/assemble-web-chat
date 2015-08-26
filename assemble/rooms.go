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
