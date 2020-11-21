package message

import(
    "time"
)

type NotifySigninData struct {
    Success bool
    Message string // Cookie if successful
}

func NewNotifySignin(success bool, message string) Server {
    return Server {
        SType: NotifySignin,
        Time: time.Now(),
        Data: NotifySigninData{
            Success: success,
            Message: message,
        },
    }
}
