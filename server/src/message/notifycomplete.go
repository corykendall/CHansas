package message

import (
    "local/hansa/simple"
)

type NotifyCompleteData struct {
    Scores []map[simple.ScoreType]int
}
