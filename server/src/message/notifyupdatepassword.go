package message

import(
    "time"
)

type NotifyUpdatePasswordData struct {
    Success bool
}

func NewNotifyUpdatePassword(success bool) Server {
    return Server {
        SType: NotifyUpdatePassword,
        Time: time.Now(),
        Data: NotifyUpdatePasswordData{
            Success: success,
        },
    }
}
