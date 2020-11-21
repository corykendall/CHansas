package message

import (
    "local/hansa/simple"
)

type NotifyEndBumpData struct {
    TurnState simple.TurnState
    Elapsed []int64
}
