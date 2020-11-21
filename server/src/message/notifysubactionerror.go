package message

import (
    "time"
)

type NotifySubactionErrorData struct {
    Header string
    Content string
}

func NewNotifySubactionError(header, content string) Server {
    return Server{
        SType: NotifySubactionError,
        Time: time.Now(),
        Data: NotifySubactionErrorData{
            Header: header,
            Content: content,
        },
    }
}
