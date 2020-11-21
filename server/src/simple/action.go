package simple

type ActionType int
const (
    NoneActionType ActionType = iota
    BagsActionType
    PlaceActionType
    BumpActionType
    MoveActionType
    ClearActionType
    SwapOfficesActionType
    LevelupActionType
    Remove3ActionType
)

type Action struct {
    Type ActionType
    Subactions []Subaction
}
