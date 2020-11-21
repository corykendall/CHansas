package message

type Broadcaster interface {
    Broadcast(Broadcast)
}

type Broadcast struct {
    Id string
    M Server
}
