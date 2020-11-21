package message

import (
    "time"
)

type InternalErrorData struct {
    Error string
}

func NewInternalError(e string) Server {
    return Server {
        SType: InternalError,
        Time: time.Now(),
        Data: InternalErrorData{
            Error: e,
        },
    }
}
