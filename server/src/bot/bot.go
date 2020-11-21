package bot

import (
    "encoding/json"
    "fmt"
    "time"
    "local/hansa/log"
    "local/hansa/message"
    "local/hansa/simple"
)

type Bot struct {
    identity simple.Identity
    brain Brain
    inMsg chan message.Server
    outMsg chan message.Client
}

func (b *Bot) Run() {
    defer b.panicking()
    for msg := range b.inMsg {
        b.dispatch(msg)
    }
}

func (b *Bot) Send(msg message.Server) {
    // deepcopy
    bytes, err := json.Marshal(msg)
    if err != nil {
        panic(fmt.Sprintf("Bot: Error marshalling, giving up: '%s' message.Server: %v", err, msg))
    }
    msg, err = message.UnmarshalServer(bytes)
    if err != nil {
        panic(fmt.Sprintf("Bot: Error unmarshalling, giving up: '%s' message.Server: %v", err, msg))
    }
    b.inMsg <- msg
}

func (b *Bot) Read() chan message.Client {
    return b.outMsg
}

func (b *Bot) Identity() simple.Identity {
    return b.identity
}

func (b *Bot) Done() {
    close(b.inMsg)
}

func (b *Bot) dispatch(m message.Server) {
    var responses []message.Client
    switch t := m.SType; t {
        case message.NotifyStartGame:
            b.brain.handleStartGame(m.Data.(message.NotifyStartGameData))
        case message.NotifySubaction:
            responses = b.brain.handleNotifySubaction(m.Data.(message.NotifySubactionData))
        case message.NotifyNextTurn:
            responses = b.brain.handleNotifyNextTurn(m.Data.(message.NotifyNextTurnData))
        case message.NotifySubactionError:
            b.brain.handleNotifySubactionError(m.Data.(message.NotifySubactionErrorData))
        case message.NotifyEndBump:
            responses = b.brain.handleNotifyEndBump(m.Data.(message.NotifyEndBumpData))
        default:
            b.log(fmt.Sprintf("Ignoring SType message.%s", t))
    }
    if responses != nil {
        for _, r := range responses {
            time.Sleep(200 * time.Millisecond)
            b.outMsg <- r
        }
    }
}

func (b *Bot) panicking() {
    if r := recover(); r != nil {
        s := fmt.Sprintf("bot panic (%v)", b)
        log.Stop(s, r)
        panic(r)
    }
}

func (b *Bot) log(msg string) {
    log.Debug("bot %s", msg)
}

func (b *Bot) fatalf(msg string, fargs ...interface{}) {
    log.Fatal(msg, fargs...)
}
