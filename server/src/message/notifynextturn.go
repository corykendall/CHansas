package message

import (
    "local/hansa/simple"
)

type NotifyNextTurnData struct {
    TurnState simple.TurnState
    Elapsed []int64
}
