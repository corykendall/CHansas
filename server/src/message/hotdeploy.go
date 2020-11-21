package message

import (
    "time"
)

type HotDeployData struct {
    Title string
    Content string
}

func NewHotDeploy(title string, content string) Server {
    return Server {
        SType: HotDeploy,
        Time: time.Now(),
        Data: HotDeployData{
            Title: title,
            Content: content,
        },
    }
}
