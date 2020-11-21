package message

import (
    "encoding/json"
    "errors"
    "fmt"
    "local/hansa/simple"
    "time"
)

type SType int
const (
    STypeNone SType = iota
    InternalError
    YourIdentity
    NotifyLobby
    NotifySignup
    NotifySignin
    NotifyPasswordReset
    NotifyConfirmEmail
    NotifyUpdatePassword
    NotifyNotification
    HotDeploy
    NotifyFullGame
    NotifyCreateGame
    NotifySitdown
    NotifyStartGame
    NotifySubaction
    NotifyNextTurn
    NotifyAction
    NotifySubactionError
    NotifyEndBump
    NotifyScoringBegin
    NotifyEndgameScoring
    NotifyComplete
)
var STypeNames = map[SType]string {
    STypeNone: "STypeNone",
    InternalError: "InternalError",
    YourIdentity: "YourIdentity",
    NotifyLobby: "NotifyLobby",
    NotifySignup: "NotifySignup",
    NotifySignin: "NotifySignin",
    NotifyPasswordReset: "NotifyPasswordReset",
    NotifyConfirmEmail: "NotifyConfirmEmail",
    NotifyUpdatePassword: "NotifyUpdatePassword",
    NotifyNotification: "NotifyNotification",
    HotDeploy: "HotDeploy",
    NotifyFullGame: "NotifyFullGame",
    NotifyCreateGame: "NotifyCreateGame",
    NotifySitdown: "NotifySitdown",
    NotifyStartGame: "NotifyStartGame",
    NotifySubaction: "NotifySubaction",
    NotifyNextTurn: "NotifyNextTurn",
    NotifyAction: "NotifyAction",
    NotifySubactionError: "NotifySubactionError",
    NotifyEndBump: "NotifyEndBump",
    NotifyScoringBegin: "NotifyScoringBegin",
    NotifyEndgameScoring: "NotifyEndgameScoring",
    NotifyComplete: "NotifyComplete",
}

func (t SType) String() string {
    return fmt.Sprintf("%s", STypeNames[t])
}

type Server struct {
    SType SType
    Time time.Time
    Data interface{}
}

func UnmarshalServer(bytes []byte) (Server, error) {
    var s Server
    err := json.Unmarshal(bytes, &s)
    if err != nil {
        return Server{}, err
    }
    var moreBytes []byte
    moreBytes, err = json.Marshal(s.Data)
    if err != nil {
        return Server{}, err
    }

    switch t := s.SType; t {
        case InternalError:
            var d InternalErrorData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case YourIdentity:
            var d YourIdentityData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyLobby:
            var d NotifyLobbyData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifySignup:
            var d NotifySignupData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifySignin:
            var d NotifySigninData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyPasswordReset:
            var d NotifyPasswordResetData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyConfirmEmail:
            var d NotifyConfirmEmailData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyUpdatePassword:
            var d NotifyUpdatePasswordData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyNotification:
            var d NotifyNotificationData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case HotDeploy:
            var d HotDeployData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyFullGame:
            var d NotifyFullGameData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyCreateGame:
            var d NotifyCreateGameData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifySitdown:
            var d NotifySitdownData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyStartGame:
            var d NotifyStartGameData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifySubaction:
            var d NotifySubactionData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyNextTurn:
            var d NotifyNextTurnData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyAction:
            var d simple.Action
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifySubactionError:
            var d NotifySubactionErrorData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyEndBump:
            var d NotifyEndBumpData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyScoringBegin:
            var d NotifyScoringBeginData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyEndgameScoring:
            var d NotifyEndgameScoringData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        case NotifyComplete:
            var d NotifyCompleteData
            err = json.Unmarshal(moreBytes, &d)
            s.Data = d
        default:
            return Server{}, errors.New(fmt.Sprintf("Unknown SType: %d", s.SType))
    }
    if err != nil {
        return Server{}, err
    }
    return s, nil
}

