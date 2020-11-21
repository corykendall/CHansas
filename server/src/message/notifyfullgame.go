package message

import (
    "local/hansa/simple"
)

type NotifyFullGameData struct {
    Status int
    Creator simple.Identity
    Table simple.Table
    TurnState simple.TurnState
    Scores []int
    FinalScores []map[simple.ScoreType]int
    Elapsed []int64
}
