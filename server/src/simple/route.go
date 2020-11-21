package simple

type Route struct {
    Id int
    Spots []Piece
    Bumped []Piece
    Token Token
    StartToken bool
    LeftCityId int
    RightCityId int
}
