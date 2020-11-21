package message

import (
    "time"
    "local/hansa/simple"
)

type YourIdentityData struct {
    Identity simple.Identity
}

func NewYourIdentity(i simple.Identity) Server {
    return Server{
        SType: YourIdentity,
        Time: time.Now(),
        Data: YourIdentityData{
            Identity: i,
        },
    }
}
