package lobby

import(
    "local/hansa/client"
    "local/hansa/simple"
)

type Player struct {
    Identity simple.Identity
    Client client.Client
}

func NewPlayer(identity simple.Identity,  client client.Client) *Player {
    return &Player{
        Identity: identity,
        Client: client,
    };
}
