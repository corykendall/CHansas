package message

import (
    "fmt"
    "errors"
    "encoding/json"
    "local/hansa/simple"
)

type CType int
const (
    CTypeNone CType = iota
    RequestSignup
    RequestSignin
    RequestPasswordReset
    UpdatePassword
    CreateGame
    RequestSitdown
    RequestSitdownBot
    StartGame
    DoSubaction
    EndTurn
    EndBump
)
var CTypeNames = map[CType]string {
    CTypeNone: "CTypeNone",
    RequestSignup: "RequestSignup",
    RequestSignin: "RequestSignin",
    RequestPasswordReset: "RequestPasswordReset",
    UpdatePassword: "UpdatePassword",
    CreateGame: "CreateGame",
    RequestSitdown: "RequestSitdown",
    RequestSitdownBot: "RequestSitdownBot",
    StartGame: "StartGame",
    DoSubaction: "DoSubaction",
    EndTurn: "EndTurn",
    EndBump: "EndBump",
}
func (t CType) String() string {
    return fmt.Sprintf("%s", CTypeNames[t])
}

func UnmarshalClient(bytes []byte) (Client, error) {
    var c Client
    err := json.Unmarshal(bytes, &c)
    if err != nil {
        return Client{}, err
    }
    var moreBytes []byte
    moreBytes, err = json.Marshal(c.Data)
    if err != nil {
        return Client{}, err
    }

    switch t := c.CType; t {
        case RequestSignup:
            var d RequestSignupData
            err = json.Unmarshal(moreBytes, &d)
            c.Data = d
        case RequestSignin:
            var d RequestSigninData
            err = json.Unmarshal(moreBytes, &d)
            c.Data = d
        case RequestPasswordReset:
            var d RequestPasswordResetData
            err = json.Unmarshal(moreBytes, &d)
            c.Data = d
        case UpdatePassword:
            var d UpdatePasswordData
            err = json.Unmarshal(moreBytes, &d)
            c.Data = d
        case CreateGame:
            var d CreateGameData
            err = json.Unmarshal(moreBytes, &d)
            c.Data = d
        case RequestSitdown:
            var d RequestSitdownData
            err = json.Unmarshal(moreBytes, &d)
            c.Data = d
        case RequestSitdownBot:
            var d RequestSitdownBotData
            err = json.Unmarshal(moreBytes, &d)
            c.Data = d
        case StartGame:
            var d StartGameData
            err = json.Unmarshal(moreBytes, &d)
            c.Data = d
        case DoSubaction:
            var d simple.Subaction
            err = json.Unmarshal(moreBytes, &d)
            c.Data = d
        case EndTurn:
            var d EndTurnData
            err = json.Unmarshal(moreBytes, &d)
            c.Data = d
        case EndBump:
            var d EndBumpData
            err = json.Unmarshal(moreBytes, &d)
            c.Data = d
        default:
            return Client{}, errors.New(fmt.Sprintf("Unknown CType: %d", c.CType))
    }
    if err != nil {
        return Client{}, err
    }
    return c, nil
}

type Client struct {
    CType CType
    Data interface{}
}
