package lobby

import (
    "fmt"
    "reflect"
    "time"
    "local/hansa/bot"
    "local/hansa/client"
    "local/hansa/database"
    "local/hansa/game"
    "local/hansa/log"
    "local/hansa/message"
    "local/hansa/simple"
    "local/hansa/user"
)

type GameJoin struct {
    C *client.WebClient
    G int
}

type Lobby struct {
    Config simple.Config
    Id int
    uh *user.Handler
    db *database.DB
    bm *bot.Manager
    ip simple.IpChecker
    broadcaster message.Broadcaster
    clients map[simple.Identity]*client.MultiWebClient
    pushTicker *time.Ticker
    join chan *client.WebClient
    gamejoin chan GameJoin
    broadcast chan message.Broadcast
    cleanupGames chan int

    // The primary thing we are a lobby for.
    games []*game.Game

    summary message.Server
}

func New(config simple.Config, uh *user.Handler, db *database.DB, bm *bot.Manager, ip simple.IpChecker, broadcaster message.Broadcaster) *Lobby {
    r := &Lobby{
        Config: config,
        Id: 1,
        uh: uh,
        db: db,
        bm: bm,
        ip: ip,
        broadcaster: broadcaster,
        clients: map[simple.Identity]*client.MultiWebClient{},
        pushTicker: time.NewTicker(5 * time.Second),
        join: make(chan *client.WebClient, 10),
        gamejoin: make(chan GameJoin, 10),
        broadcast: make(chan message.Broadcast, 10),
        cleanupGames: make(chan int),
        games: []*game.Game{},
    }
    r.refreshSummary()
    return r
}

// Call with its own goroutine to start.
func (l *Lobby) Run(initDone chan struct{}) {
    defer l.panicking()
    l.debugf("Lobby running")
    l.load()
    initDone <- struct{}{}
    for {
        l.handleMsg()
        //l.updateGames()
    }
}

func (l *Lobby) Register(c *client.WebClient) {
    l.join <- c
}

func (l *Lobby) RegisterGame(c *client.WebClient, id int) {
    l.gamejoin <- GameJoin{c, id}
}

func (l *Lobby) Broadcast(b message.Broadcast) {
    l.broadcast <-b
}

func (l *Lobby) handleMsg() {
    rcase := func(c reflect.Value) reflect.SelectCase {
        return reflect.SelectCase{
            Dir: reflect.SelectRecv,
            Chan: c,
        }
    }

    cases := []reflect.SelectCase{}
    cases = append(cases, rcase(reflect.ValueOf(l.pushTicker.C)))
    cases = append(cases, rcase(reflect.ValueOf(l.join)))
    cases = append(cases, rcase(reflect.ValueOf(l.gamejoin)))
    cases = append(cases, rcase(reflect.ValueOf(l.broadcast)))
    cases = append(cases, rcase(reflect.ValueOf(l.cleanupGames)))

    order := []simple.Identity{}
    for i, c := range l.clients {
        order = append(order, i)
        cases = append(cases, rcase(reflect.ValueOf(c.Read())))
    }

    chosen, value, ok := reflect.Select(cases)

    switch chosen {
    case 0:
        l.handleTick()
    case 1:
        l.handleJoin(value.Interface().(*client.WebClient))
    case 2:
        l.handleGameJoin(value.Interface().(GameJoin))
    case 3:
        l.handleBroadcast(value.Interface().(message.Broadcast))
    case 4:
        l.handleCleanup(value.Interface().(int))
    default:
        i := order[chosen-5]
        if !ok {
            l.handleLeave(i)
        } else {
            c := l.clients[i]
            m := value.Interface().(message.Client)
            switch ty := m.CType; ty {
                case message.CreateGame:
                    l.handleCreateGame(c, m.Data.(message.CreateGameData))
                default:
                    l.uh.Handle(c, m)
            }
        }
    }
}

func (l *Lobby) load() {
    l.infof("Load: running")
    //l.loadGames()
    l.refreshSummary()
    l.infof("load complete")
}

func (l *Lobby) handleJoin(c *client.WebClient) {
    c.Send(l.summary)
    if mc, ok := l.clients[c.Identity()]; ok {
        mc.Consume(c)
    } else {
        l.clients[c.Identity()] = client.NewMultiWebClient(c)
        go l.clients[c.Identity()].Run()
    }
}

func (l *Lobby) handleGameJoin(j GameJoin) {
    for _, g := range l.games {
        if g.Id == j.G {
            g.Register(j.C)
            return
        }
    }
    l.uh.Join(j.C)
}

func (l *Lobby) handleBroadcast(b message.Broadcast) {
    if b.Id == "" {
        for _, c := range l.clients {
            c.Send(b.M)
        }
        return
    }
    for i, c := range l.clients {
        if b.Id == i.Id {
            c.Send(b.M)
        }
        break
    }
}

func (l *Lobby) handleLeave(i simple.Identity) {
    delete(l.clients, i)
}

func (l *Lobby) handleTick() {
    l.refreshSummary()
    l.debugf("Pushing %d games to %d players", 0, len(l.clients))
    l.notify(l.summary)
}

func (l *Lobby) handleCleanup(id int) {
    l.debugf("handleCleanup for game %d", id)
    for i, g := range l.games {
        if g.Id == id {
            l.games = append(l.games[:i], l.games[i+1:]...)
            break
        }
    }
}

func (l *Lobby) handleCreateGame(c client.Client, d message.CreateGameData) {
    l.debugf("Create Game (%s)", c.Identity())

    id, err := l.db.GetNewGameId()
    if err != nil {
        panic("Unable to GetNewGameId from lobby (dynamodb)")
    }

    l.games = append([]*game.Game{game.New(id, c.Identity(), l.db, l.uh, l.bm)}, l.games...)

    initDone := make(chan struct{})
    go func() {
        l.games[0].Run(initDone)
        l.cleanupGames <- id
    }()
    <-initDone
    c.Send(message.Server{
        SType: message.NotifyCreateGame,
        Data: message.NotifyCreateGameData{
            Id: id,
        },
    })
    l.refreshSummary()
}

func (l *Lobby) refreshSummary() {
    summaries := []message.GameSummary{}
    for _, g := range l.games {
        summaries = append(summaries, g.GetSummary())
    }

    p := 0
    o := len(l.clients)
    for _, s := range summaries {
        o += s.Observers
        for _, i := range s.Players {
            if i.Type == simple.IdentityTypeConnection || i.Type == simple.IdentityTypeGuest {
                p++
            }
        }
    }
    l.summary = message.NewNotifyLobby(p, o, summaries)
}

func (l *Lobby) panicking() {
    if r := recover(); r != nil {
        log.Stop("Lobby panic", r)
        panic(r)
    }
}

func (l *Lobby) notify(m message.Server) {
    for _, p := range l.clients {
        p.Send(m)
    }
}

func (l *Lobby) debugf(msg string, fargs ...interface{}) {
    log.Debug(fmt.Sprintf("(L%d) %s", l.Id, msg), fargs...)
}

func (l *Lobby) infof(msg string, fargs ...interface{}) {
    log.Info(fmt.Sprintf("(L%d) %s", l.Id, msg), fargs...)
}

func (l *Lobby) errorf(msg string, fargs ...interface{}) {
    log.Error(fmt.Sprintf("(L%d) %s", l.Id, msg), fargs...)
}

func (l *Lobby) fatalf(msg string, fargs ...interface{}) {
    log.Fatal(fmt.Sprintf("(L%d) %s", l.Id, msg), fargs...)
}

func remove(x int, y []int) []int {
    for i, v := range y {
        if v == x {
            return append(y[:i], y[i+1:]...)
        }
    }
    return y
}

func min(x, y int) int {
    if x < y {
        return x
    }
    return y
}
