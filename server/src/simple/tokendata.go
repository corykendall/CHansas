package simple

func NewBaseStartTokens() []Token {
    return []Token{
        StartVirtualOfficeToken,
        StartSwapOfficesToken,
        StartRemove3Token,
    }
}

func NewBaseTokens() []Token {
    return []Token{
        VirtualOfficeToken,
        VirtualOfficeToken,
        VirtualOfficeToken,
        VirtualOfficeToken,
        SwapOfficesToken,
        Action3Token,
        Action3Token,
        Action4Token,
        Action4Token,
        LevelupToken,
        LevelupToken,
        LevelupToken,
        Remove3Token,
    }
}
