package message

import(
    "time"
)

type NotifyPasswordResetData struct {
    Success bool
}

func NewNotifyPasswordReset(success bool) Server {
    return Server {
        SType: NotifyPasswordReset,
        Time: time.Now(),
        Data: NotifyPasswordResetData{
            Success: success,
        },
    }
}
