package game

type Status int
const (
    StatusNone Status = iota
    Creating
    Running
    Abandoned
    Scoring
    Complete
)

