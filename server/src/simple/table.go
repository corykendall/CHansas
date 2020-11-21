package simple

import (
    "encoding/json"
    "fmt"
)

// This is what the table looks like when you unbox Hansa.  This doesn't know
// any business logic about playing the game.  If a moved piece bumps into
// another piece, they trade places (this is abused for office swapping).
// ValidateLocation and ValidateLocationAndPiece reference the contained Board
// and PlayerBoards to make sure that a location makes sense.  ApplySubaction
// mutates the Board and PlayerBoard with the given subaction; only subactions
// who's Source and Dest have been validated should be used.
type Table struct {
    Board Board
    PlayerBoards []PlayerBoard
    Scores []int
    Tokens []Token
}

// Validate that this location exists.  Idempotent.
func (t *Table) ValidateLocation(l Location) string {
    return t.validateLocation(l, false, Piece{}, NoneToken)
}

// Validate that this location and piece/token exists.  Idempotent.
func (t *Table) ValidateLocationAndPiece(l Location, p Piece, token Token) string {
    return t.validateLocation(l, true, p, token)
}

func (t *Table) ApplySubactions(ss []Subaction, i Identity) {
    for _, s := range ss {
        t.ApplySubaction(s, i)
    }
}

func (t *Table) UndoSubactions(ss []Subaction) {
    for _, s := range ss {
        if s.Source.Type == NoneLocationType || s.Dest.Type == NoneLocationType {
            fmt.Printf("NoneLocationType: %v\n", ss)
        }
    }

    for i:=len(ss)-1;i>=0;i-- {
        t.UndoSubaction(ss[i])
    }
}

func (t *Table) UndoSubaction(s Subaction) {
    t.ApplySubaction(Subaction{
        Source: s.Dest,
        Dest: s.Source,
        Piece: s.Piece,
        Token: s.Token,
    }, NewConnectionIdentity("P2342", "Undo"))
}

// Note: only call this if you have validated the Dest and validated the Source
// + Piece.  Otherwise you may explode.
func (t *Table) ApplySubaction(s Subaction, i Identity) {
    //fmt.Println("%s: %s", i, s)
    if s.Token != NoneToken {
        t.removeToken(s.Source, s.Token)
        t.addToken(s.Dest, s.Token)
    } else {
        destP := t.GetPiece(s.Dest)
        if destP == (Piece{}) {

            // Default case
            t.applyPiece(s.Source, Piece{})
            t.applyPiece(s.Dest, s.Piece)

            // Unbump
            if s.Source.Type == RouteLocationType && s.Source.Subindex == 0 {
                bumpedLocation := Location{
                    Type: s.Source.Type,
                    Id: s.Source.Id,
                    Index: s.Source.Index,
                    Subindex: 1,
                }
                bumpedP := t.GetPiece(bumpedLocation)
                if (bumpedP != Piece{}) {
                    t.applyPiece(bumpedLocation, Piece{})
                    t.applyPiece(s.Source, bumpedP)
                }
            }
            return
        }

        if s.Dest.Type == RouteLocationType && s.Dest.Subindex == 0 {
            bumpedLocation := Location{
                Type: s.Dest.Type,
                Id: s.Dest.Id,
                Index: s.Dest.Index,
                Subindex: 1,
            }
            bumpedP := t.GetPiece(bumpedLocation)

            // bump
            if (bumpedP == Piece{}) {
                t.applyPiece(s.Source, Piece{})
                t.applyPiece(s.Dest, s.Piece)
                t.applyPiece(bumpedLocation, destP)
                return
            }

            // If we are here, player tried to bump but there is something in the
            // bump zone.  Fallback to a normal swap (this should be illegal).
            panic(fmt.Sprintf("Attempted to bump but there is something in the bump spot (%s): %v", i, s))
        }

        // TODO: TEMP
        if s.Source.Type == PlayerLocationType || s.Dest.Type == PlayerLocationType {
            panic(fmt.Sprintf("Attempt to apply illegal swap (%s): (%v) (%s)", i, s, t.JsonPretty()))
        }

        // swap
        t.applyPiece(s.Source, destP)
        t.applyPiece(s.Dest, s.Piece)
    }
}

// Assumes valid Location
func (t *Table) GetPiece(l Location) Piece {
    if l.Type == RouteLocationType {
        if l.Subindex == 1 {
            return t.Board.Routes[l.Id].Bumped[l.Index]
        } 
        return t.Board.Routes[l.Id].Spots[l.Index]
    }
    if l.Type == CityLocationType {
        if l.Subindex == 2 {
            return t.Board.Cities[l.Id].Coellen.Spots[l.Index].Piece
        }
        if l.Subindex == 1 {
            return t.Board.Cities[l.Id].VirtualOffices[l.Index]
        }
        return t.Board.Cities[l.Id].Offices[l.Index].Piece
    }
    if l.Type == PlayerLocationType {
        switch l.Index {
            case 0:
                return t.PlayerBoards[l.Id].Keys[l.Subindex]
            case 1:
                return t.PlayerBoards[l.Id].Actions[l.Subindex]
            case 2:
                return t.PlayerBoards[l.Id].Priviledge[l.Subindex]
            case 3:
                return t.PlayerBoards[l.Id].Books[l.Subindex]
            case 4:
                return t.PlayerBoards[l.Id].Bags[l.Subindex]
            case 5:
                return t.PlayerBoards[l.Id].Stock[l.Subindex]
            case 6:
                return t.PlayerBoards[l.Id].Supply[l.Subindex]
        }
    }
    panic(fmt.Sprintf("GetPiece called with NoneLocation: %v", l))
}

// Assumes location is of RouteLocationType.  BFS until one empty spot is found
// starting with (and not including) this route.
func (t *Table) ValidBumps(l Location) []Location {
    open := []Location{}
    oldFrontier := []Route{t.Board.Routes[l.Id]}
    frontier := []Route{}
    for d:=1;len(open)==0;d++ {
        for _, or := range oldFrontier {
            for _, nr := range t.Board.Routes {
                if !containsRoute(nr, oldFrontier) && adjacentRoute(or, nr) {
                    frontier = append(frontier, nr)
                }
            }
        }
        for _, r := range frontier {
            for i, p := range r.Spots {
                if p == (Piece{}) {
                    open = append(open, Location{
                        Type: RouteLocationType,
                        Id: r.Id,
                        Index: i,
                        Subindex: 0,
                    })
                }
            }
        }
        oldFrontier = append(oldFrontier, frontier...)
        frontier = []Route{}
        if d > 10 {
            panic(fmt.Sprintf("ValidBumps unable to find opening from location %v.  Table: %v", l, t))
        }
    }
    return open
}

func (t *Table) Json() string {
    r, err := json.Marshal(t)
    if err != nil {
        panic(fmt.Sprintf("Unable to marshal table to json (%s): %+v", err, *t))
    }
    return string(r)
}

func (t *Table) JsonPretty() string {
    r, err := json.MarshalIndent(t, "", "  ")
    if err != nil {
        panic(fmt.Sprintf("Unable to marshal table to pretty json (%s): %+v", err, *t))
    }
    return string(r)
}

func (t *Table) validateLocation(l Location, p bool, piece Piece, token Token) string {
    if l.Type == NoneLocationType {
        return "Location has no Type (0)"
    }
    isToken := token != NoneToken
    if p {
        if isToken {
            if piece != (Piece{}) {
                return "Location specifies both Token and Piece"
            }
        } else {
            if piece.PlayerColor == NonePlayerColor {
                return "Piece does not have PlayerColor"
            }
            if piece.Shape == NoneShape {
                return "Piece does not have Shape"
            }
        }
    }
    if l.Type == RouteLocationType {
        if l.Id < 0 || l.Id >= len(t.Board.Routes) {
            return fmt.Sprintf("Route %d doesn't exist", l.Id)
        }
        if l.Index < 0 || l.Index >= len(t.Board.Routes[l.Id].Spots) {
            return fmt.Sprintf("Route Spot %d doesn't exist", l.Index)
        }
        if l.Subindex < 0 || l.Subindex > 1 {
            return fmt.Sprintf("Route invalid subindex (0=normal, 1=bumped)", l.Subindex)
        }
        if p {
            if isToken {
                if token != t.Board.Routes[l.Id].Token {
                    return "Token does not exist"
                }
            } else if l.Subindex == 1 {
                if piece != t.Board.Routes[l.Id].Bumped[l.Index] {
                    return "Bumped Piece does not exist"
                }
            } else {
                if piece != t.Board.Routes[l.Id].Spots[l.Index] {
                    return "Piece does not exist"
                }
            }
        }
    }
    if l.Type == CityLocationType {
        if l.Id < 0 || l.Id >= len(t.Board.Cities) {
            return fmt.Sprintf("City %d doesn't exist", l.Id)
        }
        if l.Subindex < 0 || l.Subindex > 2 {
            return fmt.Sprintf("City invalid subindex (0=normal, 1=virtual, 2=coellen)", l.Subindex)
        }
        if l.Subindex == 2 {
            if l.Index < 0 || t.Board.Cities[l.Id].Coellen.Spots == nil || l.Index >= len(t.Board.Cities[l.Id].Coellen.Spots) {
                return "Coellen spot doesn't exist"
            }
        } else if l.Subindex == 1 {
            if l.Index < 0 || l.Index > len(t.Board.Cities[l.Id].VirtualOffices) {
                return "Virtual Office doesn't exist and can't be created"
            }
        } else if l.Index < 0 || l.Index >= len(t.Board.Cities[l.Id].Offices) {
            return fmt.Sprintf("City Office %d doesn't exist", l.Index)
        }
        if p {
            if isToken {
                return "Tokens are not in Cities"
            } else {
                if l.Subindex == 2 {
                    if piece != t.Board.Cities[l.Id].Coellen.Spots[l.Index].Piece {
                        return "Coellen piece  does not exist"
                    }
                } else if l.Subindex == 1 {
                    if piece != t.Board.Cities[l.Id].VirtualOffices[l.Index] {
                        return "Virtual office piece does not exist"
                    }
                } else {
                    if piece != t.Board.Cities[l.Id].Offices[l.Index].Piece {
                        return "Office piece does not exist"
                    }
                }
            }
        }
    }
    if l.Type == PlayerLocationType {
        if l.Id < 0 || l.Id >= len(t.PlayerBoards) {
            return "Player does not exist"
        }
        if l.Index < 0 || l.Index > 8 {
            return "Player board section does not exist"
        }
        if l.Index == 0 && (l.Subindex < 0 || l.Subindex >= len(t.PlayerBoards[l.Id].Keys)) {
            return "That key track does not have that subindex"
        }
        if l.Index == 1 && (l.Subindex < 0 || l.Subindex >= len(t.PlayerBoards[l.Id].Actions)) {
            return "That action track does not have that subindex"
        }
        if l.Index == 2 && (l.Subindex < 0 || l.Subindex >= len(t.PlayerBoards[l.Id].Priviledge)) {
            return "That priviledge track does not have that subindex"
        }
        if l.Index == 3 && (l.Subindex < 0 || l.Subindex >= len(t.PlayerBoards[l.Id].Books)) {
            return "That books track does not have that subindex"
        }
        if l.Index == 4 && (l.Subindex < 0 || l.Subindex >= len(t.PlayerBoards[l.Id].Bags)) {
            return "That bags track does not have that subindex"
        }
        if l.Index == 5 && (l.Subindex < 0 || l.Subindex >= len(t.PlayerBoards[l.Id].Stock)) {
            return "Stock does not have that subindex"
        }
        if l.Index == 6 && (l.Subindex < 0 || l.Subindex >= len(t.PlayerBoards[l.Id].Supply)) {
            return "Supply does not have that subindex"
        }
        if p {
            if isToken {
                if l.Index < 7 {
                    return "No tokens on this section of player board"
                }
                if l.Index == 7 && !ContainsToken(t.PlayerBoards[l.Id].UnusedTokens, token) {
                    return "Token does not exist"
                }
                if l.Index == 8 && !ContainsToken(t.PlayerBoards[l.Id].UsedTokens, token) {
                    return "Token does not exist"
                }
                // TODO: Validate new Subindex field here depending on how we
                // implement tokens.
            } else {
                if l.Index == 0 && t.PlayerBoards[l.Id].Keys[l.Subindex] != piece {
                    return "Piece does not exist"
                }
                if l.Index == 1 && t.PlayerBoards[l.Id].Actions[l.Subindex] != piece {
                    return "Piece does not exist"
                }
                if l.Index == 2 && t.PlayerBoards[l.Id].Priviledge[l.Subindex] != piece {
                    return "Piece does not exist"
                }
                if l.Index == 3 && t.PlayerBoards[l.Id].Books[l.Subindex] != piece {
                    return "Piece does not exist"
                }
                if l.Index == 4 && t.PlayerBoards[l.Id].Bags[l.Subindex] != piece {
                    return "Piece does not exist"
                }
                if l.Index == 5 && t.PlayerBoards[l.Id].Stock[l.Subindex] != piece {
                    return "Piece does not exist"
                }
                if l.Index == 6 && t.PlayerBoards[l.Id].Supply[l.Subindex] != piece {
                    return "Piece does not exist"
                }
                if l.Index > 6 {
                    return "There are no cubes or discs in this board section"
                }
            }
        }
    }
    return ""
}

func (t *Table) addToken(l Location, token Token) {
    if l.Type == RouteLocationType {
        t.Board.Routes[l.Id].Token = token
    }
    if l.Type == PlayerLocationType {
        if l.Index == 7 {
            t.PlayerBoards[l.Id].UnusedTokens = append(
                t.PlayerBoards[l.Id].UnusedTokens, token)
        }
        if l.Index == 8 {
            t.PlayerBoards[l.Id].UsedTokens = append(
                t.PlayerBoards[l.Id].UsedTokens, token)
        }
    }
}

func (t *Table) removeToken(l Location, token Token) {
    if l.Type == RouteLocationType {
        t.Board.Routes[l.Id].Token = NoneToken
    }
    if l.Type == PlayerLocationType {
        if l.Index == 7 {
            t.PlayerBoards[l.Id].UnusedTokens = RemoveToken(
                t.PlayerBoards[l.Id].UnusedTokens, token)
        }
        if l.Index == 8 {
            t.PlayerBoards[l.Id].UsedTokens = RemoveToken(
                t.PlayerBoards[l.Id].UsedTokens, token)
        }
    }
}


// Assumes valid Location
func (t *Table) applyPiece(l Location, p Piece) {
    if l.Type == RouteLocationType {
        if l.Subindex == 1 {
            t.Board.Routes[l.Id].Bumped[l.Index] = p
        } else {
            t.Board.Routes[l.Id].Spots[l.Index] = p
        }
    }
    if l.Type == CityLocationType {
        if l.Subindex == 2 {
            t.Board.Cities[l.Id].Coellen.Spots[l.Index].Piece = p
        } else if l.Subindex == 1 {
            t.Board.Cities[l.Id].VirtualOffices[l.Index] = p
        } else {
            t.Board.Cities[l.Id].Offices[l.Index].Piece = p
        }
        return
    }
    if l.Type == PlayerLocationType {
        switch l.Index {
            case 0:
                t.PlayerBoards[l.Id].Keys[l.Subindex] = p
            case 1:
                t.PlayerBoards[l.Id].Actions[l.Subindex] = p
            case 2:
                t.PlayerBoards[l.Id].Priviledge[l.Subindex] = p
            case 3:
                t.PlayerBoards[l.Id].Books[l.Subindex] = p
            case 4:
                t.PlayerBoards[l.Id].Bags[l.Subindex] = p
            case 5:
                t.PlayerBoards[l.Id].Stock[l.Subindex] = p
            case 6:
                t.PlayerBoards[l.Id].Supply[l.Subindex] = p
        }
    }
}

func containsRoute(r Route, rs []Route) bool {
    for _, r2 := range rs {
        if r.Id == r2.Id {
            return true
        }
    }
    return false
}

func adjacentRoute(r1 Route, r2 Route) bool {
    return r1.LeftCityId == r2.LeftCityId ||
        r1.LeftCityId == r2.RightCityId ||
        r1.RightCityId == r2.LeftCityId ||
        r1.RightCityId == r2.RightCityId
}
