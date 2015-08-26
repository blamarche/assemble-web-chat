package assemble

import (
	"time"

	"github.com/googollee/go-socket.io"
)

// OnlineUser has info for and online user including last active timestamp
type OnlineUser struct {
	So      *socketio.Socket
	LastAct time.Time
}
