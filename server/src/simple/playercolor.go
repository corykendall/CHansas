package simple

type PlayerColor int
const (
    NonePlayerColor PlayerColor = iota
    YellowPlayerColor
    GreenPlayerColor
    BluePlayerColor
    PurplePlayerColor
    RedPlayerColor
)

var PlayerColorNames = map[PlayerColor]string{
    NonePlayerColor: "None",
    YellowPlayerColor: "Yellow",
    GreenPlayerColor: "Green",
    BluePlayerColor: "Blue",
    PurplePlayerColor: "Purple",
    RedPlayerColor: "Red",
}
