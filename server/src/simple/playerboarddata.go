package simple

func NewPlayerBoard(i Identity, c PlayerColor) PlayerBoard {
    stock := []Piece{}
    supply := []Piece{}
    for i:=0;i<30;i++ {
        stock = append(stock, Piece{})
        supply = append(supply, Piece{})
    }

    return PlayerBoard{
        Identity: i,
        Color: c,
        UnusedTokens: []Token{},
        UsedTokens: []Token{},
        Stock: stock,
        Supply: supply,
        Actions: []Piece{Piece{}, Piece{c, CubeShape}, Piece{c, CubeShape}, Piece{c, CubeShape}, Piece{c, CubeShape}, Piece{c, CubeShape}},
        Books: []Piece{Piece{}, Piece{c, DiscShape}, Piece{c, DiscShape}, Piece{c, DiscShape}},
        Priviledge: []Piece{Piece{}, Piece{c, CubeShape}, Piece{c, CubeShape}, Piece{c, CubeShape}},
        Bags: []Piece{Piece{}, Piece{c, CubeShape}, Piece{c, CubeShape}, Piece{c, CubeShape}},
        Keys: []Piece{Piece{}, Piece{c, CubeShape}, Piece{c, CubeShape}, Piece{c, CubeShape}, Piece{c, CubeShape}},
    }
}

func NewBasePlayerBoards() []PlayerBoard {
    r := []PlayerBoard{}
    for i:=0;i<5;i++ {
        r = append(r, NewPlayerBoard(EmptyIdentity, PlayerColor(i+1)))
    }
    return r
}


