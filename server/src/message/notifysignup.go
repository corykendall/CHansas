package message

import(
    "time"
)

type NotifySignupData struct {
    Success bool
    Message string
}

func NewNotifySignup(success bool, message string) Server {
    return Server {
        SType: NotifySignup,
        Time: time.Now(),
        Data: NotifySignupData{
            Success: success,
            Message: message,
        },
    }
}
