package message

import (
    "time"
    "local/hansa/simple"
)

type NotifyLobbyData struct {
    Players int
    Observers int
    Games []GameSummary
}

func NewNotifyLobby(players int, observers int, games []GameSummary) Server {
    return Server {
        SType: NotifyLobby,
        Time: time.Now(),
        Data: NotifyLobbyData{
            Players: players,
            Observers: observers,
            Games: games,
        },
    }
}

type GameSummary struct {
    Id int
    RunningTime time.Time
    CompleteTime time.Time
    Status int
    Creator simple.Identity
    Players []simple.Identity
    Colors []simple.PlayerColor
    Scores []int
    Observers int
}
