package server

import (
    "local/hansa/lobby"
    "local/hansa/message"
    "local/hansa/user"
)

// Implements message.Broadcaster interface
type Broadcaster struct {
    lobby *lobby.Lobby
    uh *user.Handler
}

func NewBroadcaster() *Broadcaster {
    return &Broadcaster{}
}

// Notifies all websockets we can find for this identity id b.Id (or every
// identity if "") with a message (generally a notification).
func (bc *Broadcaster) Broadcast(b message.Broadcast) {
    bc.lobby.Broadcast(b)
    bc.uh.Broadcast(b)
}
