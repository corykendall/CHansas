package simple

import (
    "time"
)

type TurnStateType int
const (
    NoneTurnStateType TurnStateType = iota // Implemented
    Bags // Implemented
    BumpPaying // Implemented
    Bumping // Implemented
    Moving // Implemented
    Clearing // In Progress
    Remove3
    LevelUp
    BonusOffice
    SwapOffice
)
var NoneTurnState = TurnState {Type: NoneTurnStateType}

type TurnState struct {
    Type TurnStateType

    // Used for everything
    Player int
    ActionsLeft int
    TurnStart time.Time
    TurnElapsedDelta int64

    // If we bump someone, we set BumpingStart and UIs know that the elapsed
    // turn time is the difference between TurnStart and BumpingStart.  When
    // the bumping ends, server sets TurnStart to time.Now and adds that
    // difference to TurnElapsedDelta.  UIs can now display the elapsed time
    // from TurnStart to now + the TurnElapsedDelta.  This works for future
    // bumps in the same turn as well.

    // Used for Bags
    BagsLeft int

    // Used for BumpPaying
    BumpPayingCost int

    // Used for Bumping
    BumpingStart time.Time
    BumpingPlayer int
    BumpingLocation Location
    BumpingMoved bool
    BumpingReplaces int

    // Used for Moving
    MovesLeft int

    // Used for Clearing
    ClearingAward Award
    ClearingRouteId int 
    ClearingCanOffice bool
}


