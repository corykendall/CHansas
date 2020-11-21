package simple

import (
    "fmt"
    "strconv"
)

type IdentityType int
const (
    IdentityTypeNone IdentityType = iota
    IdentityTypeConnection
    IdentityTypeGuest
    IdentityTypeBot
)
var IdentityTypeNames = map[IdentityType]string {
    IdentityTypeNone: "IdentityTypeNone",
    IdentityTypeConnection: "IdentityTypeConnection",
    IdentityTypeGuest: "IdentityTypeGuest",
    IdentityTypeBot: "IdentityTypeBot",
}

// Id prefix: G = Guest, P = Player, B = Bot, "" is the EmptyIdentity.
type Identity struct {
    Id string
    Name string
    Type IdentityType
}

func NewIdentity(id string, name string, idtype IdentityType) Identity {
    return Identity{
        Id: id,
        Name: name,
        Type: idtype,
    }
}

func NewConnectionIdentity(id string, name string) Identity {
    return NewIdentity(id, name, IdentityTypeConnection)
}
func NewGuestIdentity(id string) Identity {
    number, _ := strconv.Atoi(id[1:])
    return NewIdentity(id, fmt.Sprintf("Guest%d", number), IdentityTypeGuest)
}
func NewBotIdentity(id string, name string) Identity {
    return NewIdentity(id, name, IdentityTypeBot)
}

var EmptyIdentity = NewIdentity("", "", IdentityTypeNone)

func (a Identity) String() string {
    return fmt.Sprintf("(I:%s-%s)", a.Id, a.Name)
}
