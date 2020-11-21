package simple

type Token int
const (
    NoneToken Token = iota
    StartVirtualOfficeToken
    StartSwapOfficesToken
    StartRemove3Token
    VirtualOfficeToken
    SwapOfficesToken
    Action3Token
    Action4Token
    LevelupToken
    Remove3Token
)

func ContainsToken(ts []Token, x Token) bool {
    for _, t := range ts {
        if x == t {
            return true
        }
    }
    return false
}

func RemoveToken(ts []Token, x Token) []Token {
    for i, t := range ts {
        if x == t {
            return append(ts[:i], ts[i+1:]...)
        }
    }
    return ts
}
