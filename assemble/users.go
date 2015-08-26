package assemble

import (
	"time"

	"github.com/Jeffail/gabs"
	"github.com/googollee/go-socket.io"
)

// OnlineUser has info for and online user including last active timestamp
type OnlineUser struct {
	So       *socketio.Socket
	LastPing time.Time
}

// AlertState holds info about the last alert that was sent to an offline user
type User struct {
	LastAlert time.Time
	LastAct   time.Time
	Token     *gabs.Container
}
