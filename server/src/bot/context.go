package bot

// This is a little flawed right now.  It's safer to trust the board state and
// recount when you need these values, because the board state changes with
// every move, and we dno't want to recalc the whole context when your first
// play is to play a disc on a ruote and the second play checks the context for
// "SupplyDisc" it will be wrong.  Maybe these should be moved to methods in
// the bots themselves that calculate from live data on demand.
type Context struct {

    // Number of actions I have
    ActionsLeft int

    // My pieces in stock, supply, or on the board
    LivePieces int
    StockPieces int
    SupplyPieces int
    BoardPieces int

    // If I have a discs in these locations
    StockDisc bool
    SupplyDisc bool
    BoardDisc bool

    // This effects things like Coellen (want late, not early) and offices over
    // upgrades
    GameTime GameTime
}

type GameTime int
const (
    EarlyGame GameTime = iota
    MidGame
    LateGame
)
