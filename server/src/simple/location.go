package simple

type LocationType int
const (
    NoneLocationType LocationType = iota
    RouteLocationType
    CityLocationType
    PlayerLocationType
)
var NoneLocation = Location{Type: NoneLocationType}

type Location struct {
    Type LocationType
    Id int
    Index int

    // City: 0=normal, 1=virtual, 2=coellen.  Route: 0=normal, 1=bumped.
    // Player:
    //     0: Keys
    //     1: Actions
    //     2: Privilidgium
    //     3: Books
    //     4: Bags
    //     5: Stock (you have to bags from here)
    //     6: Supply (where you play from)
    //     7: Token Unused
    //     8: Token Used
    Subindex int
}

