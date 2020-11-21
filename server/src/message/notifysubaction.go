package message

import (
    "local/hansa/simple"
)

type NotifySubactionData struct {
    // This is what just happened.  Clients should be able to apply this
    // Subaction directly to their Table immediately without validation.
    Subaction simple.Subaction

    // These are only score deltas as a result of this subaction
    Scores []int

    // This defines what we are waiting for next. 
    TurnState simple.TurnState

    // This means that the game is ending after this Action completes.
    Gameend bool
}



