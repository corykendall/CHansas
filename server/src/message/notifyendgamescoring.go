package message

import (
    "local/hansa/simple"
)

type NotifyEndgameScoringData struct {
    Player int
    Type simple.ScoreType
    Score int
}
