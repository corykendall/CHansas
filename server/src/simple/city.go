package simple

type City struct {
    Id int
    Name string
    Offices []Office
    VirtualOffices []Piece
    Coellen CoellenTable
    Award Award
    BonusTerminus bool
}

func (c City) GetPresence(color PlayerColor) int {
    r := 0
    for _, o := range c.Offices {
        if o.Piece.PlayerColor == color {
            r++
        }
    }
    for _, o := range c.VirtualOffices {
        if o.PlayerColor == color {
            r++
        }
    }
    return r
}

func (c City) GetControl() PlayerColor {
    m := map[PlayerColor]int {
        YellowPlayerColor: 0,
        GreenPlayerColor: 0,
        BluePlayerColor: 0,
        PurplePlayerColor: 0,
        RedPlayerColor: 0,
    }

    found := false
    for _, o := range c.Offices {
        if o.Piece != (Piece{}) {
            m[o.Piece.PlayerColor] = m[o.Piece.PlayerColor]+1
            found = true
        }
    }
    for _, p := range c.VirtualOffices {
        if p != (Piece{}) {
            m[p.PlayerColor] = m[p.PlayerColor]+1
            found = true
        }
    }
    if !found {
        return NonePlayerColor
    }

    max := 0
    winners := []PlayerColor{}
    for color, v := range m {
        if v > max {
            max = v
            winners = []PlayerColor{color}
        } else if v == max {
            winners = append(winners, color)
        }
    }

    if len(winners) == 1 {
        return winners[0]
    } 
    for i:=len(c.Offices)-1;i>=0;i-- {
        color := c.Offices[i].Piece.PlayerColor
        if containsColor(color, winners) {
            return color
        }
    }
    for i:=len(c.VirtualOffices)-1;i>=0;i-- {
        color := c.VirtualOffices[i].PlayerColor
        if containsColor(color, winners) {
            return color
        }
    }

    // Some kind of algorithm fuckup
    return NonePlayerColor
}

func containsColor(c PlayerColor, cs []PlayerColor) bool {
    for _, c2 := range cs {
        if c == c2 {
            return true
        }
    }
    return false
}
