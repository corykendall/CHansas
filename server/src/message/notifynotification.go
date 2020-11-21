package message

import (
    "fmt"
    "time"
)

type NotificationType int
const (
    NotificationError NotificationType = iota
    NotificationWarn
    NotificationInfo
    NotificationSuccess
    NotificationInternalError
)

var NotificationTypeNames = map[NotificationType]string {
    NotificationError: "Error",
    NotificationWarn: "Warn",
    NotificationInfo: "Info",
    NotificationSuccess: "Success",
    NotificationInternalError: "InternalError",
}

func (a NotificationType) String() string {
    return fmt.Sprintf("%s", NotificationTypeNames[a])
}

type NotifyNotificationData struct {
    Type NotificationType
    Header string
    Content string
}

func NewNotifyNotification(Type NotificationType, header string, content string) Server {
    return Server {
        SType: NotifyNotification,
        Time: time.Now(),
        Data: NotifyNotificationData {
            Type: Type,
            Header: header,
            Content: content,
        },
    }
}
