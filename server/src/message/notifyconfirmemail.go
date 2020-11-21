package message

import (
    "time"
)

type NotifyConfirmEmailData struct {
    Success bool
}

func NewNotifyConfirmEmail(s bool) Server {
    return Server {
        SType: NotifyConfirmEmail,
        Time: time.Now(),
        Data: NotifyConfirmEmailData{
            Success: s,
        },
    }
}
