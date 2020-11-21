package bot

import (
    "local/hansa/message"
)

type Brain interface {
    handleStartGame(d message.NotifyStartGameData)
    handleNotifySubaction(d message.NotifySubactionData) []message.Client
    handleNotifyNextTurn(d message.NotifyNextTurnData) []message.Client
    handleNotifySubactionError(d message.NotifySubactionErrorData)
    handleNotifyEndBump(d message.NotifyEndBumpData) []message.Client
    // handleUndo
    // handleEndTurn
}

