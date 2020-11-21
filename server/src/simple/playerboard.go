package simple

type PlayerBoard struct {
    Identity Identity
    Color PlayerColor

    // Note these are referenced by Location.Index, and then into the
    // underlying slices with Location.Subindex
    UnusedTokens []Token
    UsedTokens []Token
    Stock []Piece
    Supply []Piece
    Keys []Piece
    Priviledge []Piece
    Books []Piece
    Actions []Piece
    Bags []Piece
}

func (p PlayerBoard) GetActions() int {
    l := left(p.Actions)
    if l == 5 {
        return 2
    } else if l == 4 {
        return 3
    } else if l == 3 {
        return 3
    } else if l == 2 {
        return 4
    } else if l == 1 {
        return 4
    }
    return 5
}

func (p PlayerBoard) GetBooks() int {
    return 5 - left(p.Books)
}

func (p PlayerBoard) GetPriviledge() Priviledge {
    return Priviledge(4 - left(p.Priviledge))
}

func (p PlayerBoard) GetKeys() int {
    l := left(p.Keys)
    if l == 4 {
        return 1
    } else if l == 3 {
        return 2
    } else if l == 2 {
        return 2
    } else if l == 1 {
        return 3
    }
    return 4
}

// 3, 5, 7, 100
func (p PlayerBoard) GetBags() int {
    r := 3 + ((3 - left(p.Bags))*2)
    if r == 9 {
        return 100
    }
    return r
}

func (p PlayerBoard) GetActionCubes() int {
    return left(p.Actions)
}
func (p PlayerBoard) GetBookDiscs() int {
    return left(p.Books)
}
func (p PlayerBoard) GetPriviledgeCubes() int {
    return left(p.Priviledge)
}
func (p PlayerBoard) GetKeyCubes() int {
    return left(p.Keys)
}
func (p PlayerBoard) GetBagCubes() int {
    return left(p.Bags)
}

func (p PlayerBoard) GetLeftmostActionCube() int {
    return leftmost(p.Actions)
}
func (p PlayerBoard) GetLeftmostBookDisc() int {
    return leftmost(p.Books)
}
func (p PlayerBoard) GetLeftmostPriviledgeCube() int {
    return leftmost(p.Priviledge)
}
func (p PlayerBoard) GetLeftmostKeyCube() int {
    return leftmost(p.Keys)
}
func (p PlayerBoard) GetLeftmostBagCube() int {
    return leftmost(p.Bags)
}

// Note CoellenAward must be handled separately (this will return false)
func (p PlayerBoard) CanAward(a Award) bool {
    switch a {
        case DiscsAward:
            return p.GetBookDiscs() > 0
        case PriviledgeAward:
            return p.GetPriviledgeCubes() > 0
        case BagsAward:
            return p.GetBagCubes() > 0
        case ActionsAward:
            return p.GetActionCubes() > 0
        case KeysAward:
            return p.GetKeyCubes() > 0
    }
    return false
}

// Note CoellenAward must be handled separately (this will return false)
func (p PlayerBoard) AwardTrackRemaining(a Award) int {
    switch a {
        case DiscsAward:
            return p.GetBookDiscs()
        case PriviledgeAward:
            return p.GetPriviledgeCubes()
        case BagsAward:
            return p.GetBagCubes()
        case ActionsAward:
            return p.GetActionCubes()
        case KeysAward:
            return p.GetKeyCubes()
    }
    return 0
}

// Note CoellenAward must be handled separately (this will return 0, 0).
// Otherwise, this returns an Index/Subindex for how to clear an award.
func (p PlayerBoard) AwardClearLocation(a Award) (int, int) {
    switch a {
        case DiscsAward:
            return 3, p.GetLeftmostBookDisc()
        case PriviledgeAward:
            return 2, p.GetLeftmostPriviledgeCube()
        case BagsAward:
            return 4, p.GetLeftmostBagCube()
        case ActionsAward:
            return 1, p.GetLeftmostActionCube()
        case KeysAward:
            return 0, p.GetLeftmostKeyCube()
    }
    return 0, 0
}

func left(track []Piece) (l int) {
    for _, p := range track {
        if p != (Piece{}) {
            l++
        }
    }
    return
}

func leftmost(track []Piece) int {
    for i, p := range track {
        if p != (Piece{}) {
            return i
        }
    }
    return 100
}

