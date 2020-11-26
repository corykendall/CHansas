package game

import (
    "fmt"
    "math/rand"
    "reflect"
    "sort"
    "sync"
    "time"
    "local/hansa/bot"
    "local/hansa/client"
    "local/hansa/database"
    "local/hansa/log"
    "local/hansa/message"
    "local/hansa/simple"
    "local/hansa/user"
)

type Game struct {

    // Immutable fields
    Id int
    Creator simple.Identity

    // Other heavy components we interface with
    db *database.DB
    uh *user.Handler
    bm *bot.Manager

    // The way that other components talk to us
    joins chan client.Client
    broadcast chan message.Broadcast
    scoring chan message.Server

    // The way users talk to us.  During Creating, players is uninitialized and
    // all communication is through observers.  Once we transition, players is
    // initialized to the right players.
    observers map[simple.Identity]*client.MultiWebClient
    players []*Player // turn order
    disconnects map[int]bool

    // Lifecycle
    status Status
    newStatus Status
    times GameTimes
    timeouts chan TimeoutType
    summaryMux sync.Mutex
    summary message.GameSummary

    // Game state
    table *simple.Table
    turnState simple.TurnState
    scores []int
    bonusroute []bool
    gameend bool
    finalscores []map[simple.ScoreType]int

    // Game History (also used for undo)
    turns []simple.Turn
    actions []simple.Action // only for uncompleted current turn, may be undone.
    subactions []simple.Subaction // only for uncompleted current turn, may be undone.
}

type GameTimes struct {
    create time.Time
    running time.Time
    elapsed []time.Duration
    complete time.Time
}

func New(id int, creator simple.Identity, db *database.DB, uh *user.Handler, bm *bot.Manager) *Game {
    return &Game{
        Id: id,
        Creator: creator,
        db: db,
        uh: uh,
        bm: bm,
        joins: make(chan client.Client, 2),
        broadcast: make(chan message.Broadcast, 10),
        scoring: make(chan message.Server, 10),
        observers: map[simple.Identity]*client.MultiWebClient{},
        disconnects: map[int]bool{},
        status: Creating,
        newStatus: Creating,
        times: GameTimes{create: time.Now(), elapsed: []time.Duration{0, 0, 0, 0, 0}},
        timeouts: make(chan TimeoutType),
        summaryMux: sync.Mutex{},
        table: &simple.Table{
            Board: simple.NewBase45Board(),
            PlayerBoards: simple.NewBasePlayerBoards(),
            Tokens: simple.NewBaseTokens(),
        },
        scores: []int{0, 0, 0, 0, 0},
    }
}

// TODO: Load
func Load() {}

func (g *Game) Run(initDone chan struct{}) {
    defer g.panicking()
    g.hotdeployLoad()
    //g.dbStore()
    g.updateSummary()
    initDone <- struct{}{}

    for ;g.handleMsg(); {
        g.checkStatus()
        g.updateSummary()
    }
    g.checkStatus()
    g.updateSummary()
}

func (g *Game) Register(c client.Client) {
    g.joins <-c
}

func (g *Game) GetSummary() message.GameSummary {
    g.summaryMux.Lock()
    defer func() { g.summaryMux.Unlock() }()
    return g.summary
}

func (g *Game) updateSummary() {
    g.summaryMux.Lock()
    defer func() { g.summaryMux.Unlock() }()

    is := []simple.Identity{}
    cs := []simple.PlayerColor{}
    for _, pb := range g.table.PlayerBoards {
        is = append(is, pb.Identity)
        cs = append(cs, pb.Color)
    }

    g.summary = message.GameSummary{
        Id: g.Id,
        RunningTime: g.times.running,
        CompleteTime: g.times.complete,
        Status: int(g.status),
        Creator: g.Creator,
        Players: is,
        Colors: cs,
        Scores: g.scores,
        Observers: len(g.observers),
    }
}

// Returns false if this game is over (Abandoned, Complete) and there are no
// observers.
func (g *Game) handleMsg() bool {
    rcase := func(c reflect.Value) reflect.SelectCase {
        return reflect.SelectCase{
            Dir: reflect.SelectRecv,
            Chan: c,
        }
    }

    obOrder := []simple.Identity{}
    cases := []reflect.SelectCase{
        rcase(reflect.ValueOf(g.joins)),
        rcase(reflect.ValueOf(g.timeouts)),
        rcase(reflect.ValueOf(g.broadcast)),
        rcase(reflect.ValueOf(g.scoring)),
    }
    for i, p := range g.players {
        c := p.Client
        if v, ok := g.disconnects[i]; ok && v {
            c = client.EmptyClient{}
        }
        cases = append(cases, rcase(reflect.ValueOf(c.Read())))
    }
    for i, o := range g.observers {
        obOrder = append(obOrder, i)
        cases = append(cases, rcase(reflect.ValueOf(o.Read())))
    }

    // A closed channel here means that the player/observer has 0 open
    // connections to the table.
    chosen, value, ok := reflect.Select(cases)
    if chosen == 0 {
        if !ok {
            panic("g.joins should never be closed!")
        }
        g.handleJoin(value.Interface().(*client.WebClient))
    } else if chosen == 1 {
        if !ok {
            panic("g.timeouts should never be closed!")
        }
        g.handleTimeout(value.Interface().(TimeoutType))
    } else if chosen == 2 {
        if !ok {
            panic("g.broadcast should never be closed!")
        }
        g.handleBroadcast(value.Interface().(message.Broadcast))
    } else if chosen == 3 {
        if !ok {
            panic("g.scoring should never be closed!")
        }
        g.handleScoring(value.Interface().(message.Server))
    } else if len(g.players) > chosen-4 {
        if !ok {
            p := g.players[chosen-4]
            g.debugf("Player %s (%s) disconnected", p.Client.Identity())
            g.disconnects[chosen-4] = true
        } else {
            g.handlePlayerMsg(chosen-4, g.players[chosen-4], value.Interface().(message.Client))
        }
    } else {
        if !ok {
            delete(g.observers, obOrder[(chosen-4)-len(g.players)])
        } else {
            g.handleObserverMsg(g.observers[obOrder[(chosen-4)-len(g.players)]],
                value.Interface().(message.Client))
        }
    }

    // Return false if we should get rid of this game instance.
    if g.status != Abandoned && g.status != Complete || len(g.observers) > 0 {
        return true
    }
    for i, _ := range g.players {
        if v, ok := g.disconnects[i]; !ok || !v {
            return true
        }
    }
    return false
}

func (g *Game) castElapsed() (r []int64) {
    for _, d := range g.times.elapsed {
        r = append(r, int64(d))
    }
    return
}

func (g *Game) handleJoin(c *client.WebClient) {
    g.debugf("HandleJoin %s", c.Identity())

    // This won't include the history, but the user can ask for it separately.
    c.Send(message.Server{
        SType: message.NotifyFullGame,
        Time: time.Now(),
        Data: message.NotifyFullGameData{
            Status: int(g.status),
            Creator: g.Creator,
            Table: *g.table,
            TurnState: g.turnState,
            Elapsed: g.castElapsed(),
            Scores: g.scores,
            FinalScores: g.finalscores,
        },
    })

    // Look for this identity as a player or an observer.
    if mc, ok := g.observers[c.Identity()]; ok {
        g.debugf("Already an observer, consuming new ws: %s", c.Identity().Id)
        mc.Consume(c)
        return
    }
    for i, p := range g.players {
        if p.Client.Identity() == c.Identity() {
            g.debugf("Already a player, consuming new ws: %s", c.Identity().Id)
            p.Client.(*client.MultiWebClient).Consume(c)
            delete(g.disconnects, i)
            return
        }
    }

    // If not found, a new Observer.
    g.debugf("New Observer: %s", c.Identity().Id)
    mc := client.NewMultiWebClient(c)
    go mc.Run()
    g.observers[c.Identity()] = mc
}

func (g *Game) handleTimeout(tt TimeoutType) {
    // TODO: this
}

func (g *Game) handleBroadcast(b message.Broadcast) {
    // TODO: this
}

func (g *Game) handleScoring(m message.Server) {
    m.Time = time.Now()
    if m.SType == message.NotifyComplete {
        m.Data = message.NotifyCompleteData{
            Scores: g.finalscores,
        }
        g.newStatus = Complete
        g.times.complete = time.Now()
        for i, _ := range g.scores {
            g.scores[i] = g.finalscores[i][simple.TotalScoreType]
        }
    }
    if m.SType == message.NotifyEndgameScoring {
        d := m.Data.(message.NotifyEndgameScoringData)
        g.finalscores[d.Player][d.Type] = d.Score
    }
    g.notify(m)
}

func (g *Game) handlePlayerMsg(i int, p *Player, m message.Client) {
    switch ty := m.CType; ty {
        case message.RequestSignup:
            g.uh.Handle(p.Client, m)
        case message.RequestSignin:
            g.uh.Handle(p.Client, m)
        case message.RequestPasswordReset:
            g.uh.Handle(p.Client, m)
        case message.UpdatePassword:
            g.uh.Handle(p.Client, m)
        case message.RequestSitdown:
            g.handleRequestSitdown(p.Client, m.Data.(message.RequestSitdownData))
        case message.RequestSitdownBot:
            g.handleRequestSitdownBot(p.Client, m.Data.(message.RequestSitdownBotData))
        case message.StartGame:
            g.handleStartGame(p.Client, m.Data.(message.StartGameData))
        case message.DoSubaction:
            g.handleDoSubaction(i, p.Client, m.Data.(simple.Subaction))
        case message.EndTurn:
            g.handleEndTurn(i, p.Client, m.Data.(message.EndTurnData))
        case message.EndBump:
            g.handleEndBump(i, p.Client, m.Data.(message.EndBumpData))
        default:
            g.clientError(p.Client, "Client Error", "CType '%s' unhandled by Game (player)",
                message.CTypeNames[m.CType])
    }
}

func (g *Game) handleObserverMsg(o *client.MultiWebClient, m message.Client) {
    switch ty := m.CType; ty {
        case message.RequestSignup:
            g.uh.Handle(o, m)
        case message.RequestSignin:
            g.uh.Handle(o, m)
        case message.RequestPasswordReset:
            g.uh.Handle(o, m)
        case message.UpdatePassword:
            g.uh.Handle(o, m)
        case message.RequestSitdown:
            g.handleRequestSitdown(o, m.Data.(message.RequestSitdownData))
        case message.RequestSitdownBot:
            g.handleRequestSitdownBot(o, m.Data.(message.RequestSitdownBotData))
        case message.StartGame:
            g.handleStartGame(o, m.Data.(message.StartGameData))
        default:
            g.clientError(o, "Client Error", "CType '%s' unhandled by Game (observer)",
                message.CTypeNames[m.CType])
    }
}

func (g *Game) handleRequestSitdown(c client.Client, d message.RequestSitdownData) {
    if g.status != Creating {
        g.clientError(c, "Sitdown Error", "You can only stand up when a game is 'Creating'")
        return
    }

    isSitting := false
    for _, b := range g.table.PlayerBoards {
        if b.Identity == c.Identity() {
            isSitting = true
            break
        }
    }
    if isSitting && d.Sitdown {
        g.clientError(c, "Sitdown Error", "You are already sitting at this game")
        return
    }
    if !isSitting && !d.Sitdown {
        g.clientError(c, "Sitdown Error", "You can not stand up: you are not sitting at this game")
        return
    }
    if d.Index < 0 || d.Index >= 5 {
        g.clientError(c, "Sitdown Error", "Not a valid seat (expecting [0-4])")
        return
    }

    i := g.table.PlayerBoards[d.Index].Identity
    if d.Sitdown {
        if i != simple.EmptyIdentity {
            g.clientError(c, "Sitdown Error", "%s is already there", i.Name)
            return
        }
        g.debugf("(%s) Sat down", c.Identity())
        g.table.PlayerBoards[d.Index].Identity = c.Identity()
        g.notify(message.Server{
            SType: message.NotifySitdown,
            Time: time.Now(),
            Data: message.NotifySitdownData{
                Identity: c.Identity(),
                Index: d.Index,
                Sitdown: true,
            },
        })
        return
    }

    if i != c.Identity() {
        g.clientError(c, "Sitdown Error", "You are not sitting there")
        return
    }
    g.debugf("(%s) Stood up", c.Identity())
    g.table.PlayerBoards[d.Index].Identity = simple.EmptyIdentity
    g.notify(message.Server{
        SType: message.NotifySitdown,
        Time: time.Now(),
        Data: message.NotifySitdownData{
            Identity: c.Identity(),
            Index: d.Index,
            Sitdown: false,
        },
    })
}

func (g *Game) handleRequestSitdownBot(c client.Client, d message.RequestSitdownBotData) {
    identity := g.bm.GetIdentity(d.Id)
    if identity == simple.EmptyIdentity {
        g.clientError(c, "Sitdown Error", "Not a valid Bot ID: '%s'", d.Id)
        return
    }
    if g.status != Creating {
        g.clientError(c, "Sitdown Error", "A Bot can only sit down when a game is 'Creating'")
        return
    }
    if c.Identity() != g.Creator {
        g.clientError(c, "Sitdown Error", "Only the game creator (%s) may add/remove bots", g.Creator.Name)
        return
    }

    isSitting := false
    for _, b := range g.table.PlayerBoards {
        if b.Identity == identity {
            isSitting = true
            break
        }
    }
    if isSitting && d.Sitdown {
        g.clientError(c, "Sitdown Error", "%s is already sitting at this game", identity)
        return
    }
    if !isSitting && !d.Sitdown {
        g.clientError(c, "Sitdown Error", "%s can not stand up: not sitting at this game", identity)
        return
    }
    if d.Index < 0 || d.Index >= 5 {
        g.clientError(c, "Sitdown Error", "Seat does not exist: %d", d.Index)
        return
    }

    i := g.table.PlayerBoards[d.Index].Identity
    if d.Sitdown {
        if i != simple.EmptyIdentity {
            g.clientError(c, "Sitdown Error", "%s is already there", i.Name)
            return
        }
        g.debugf("(%s) Sat down", identity)
        g.table.PlayerBoards[d.Index].Identity = identity
        g.notify(message.Server{
            SType: message.NotifySitdown,
            Time: time.Now(),
            Data: message.NotifySitdownData{
                Identity: identity,
                Index: d.Index,
                Sitdown: true,
            },
        })
        return
    }

    if i != identity {
        g.clientError(c, "Sitdown Error", "Bot is not sitting there")
        return
    }
    g.debugf("(%s) Stood up", identity)
    g.table.PlayerBoards[d.Index].Identity = simple.EmptyIdentity
    g.notify(message.Server{
        SType: message.NotifySitdown,
        Time: time.Now(),
        Data: message.NotifySitdownData{
            Identity: identity,
            Index: d.Index,
            Sitdown: false,
        },
    })
}

func (g *Game) handleStartGame(c client.Client, d message.StartGameData) {
    if g.status != Creating {
        g.clientError(c, "StartGame Error", "Can only start when game is 'Creating'")
        return
    }
    if c.Identity() != g.Creator {
        g.clientError(c, "StartGame Error", "Only the Creator (%s) can start the game", g.Creator.Name)
        return
    }

    players := 0
    for _, pb := range g.table.PlayerBoards {
        if pb.Identity != simple.EmptyIdentity {
            players++
        }
    }
    if players < 4 {
        g.clientError(c, "StartGame Error", "Only 4-5 players is supported :(")
        return
    }

    g.debugf("Starting Game")
    g.newStatus = Running
}

// Validate that the Locations are valid, the piece is present at source, the
// source and dest aren't identical, and it's my turn to make an action.  No
// business logic (other than turn) validated.
func (g *Game) validateSubaction(p int, c client.Client, d simple.Subaction) bool {
    if g.status != Running {
        g.subactionError(c, "Subaction Error", "Can only move pieces when game is 'Running'")
        return false
    }
    if g.turnState.Type == simple.Bumping && p != g.turnState.BumpingPlayer {
        g.subactionError(c, "Subaction Error", "It's %s's turn to replace after the bump",
            g.players[g.turnState.BumpingPlayer].Client.Identity().Name,
        )
        return false
    }
    if g.turnState.Type != simple.Bumping && p != g.turnState.Player {
        g.subactionError(c, "Subaction Error", "It's not your turn")
        return false
    }

    err := g.table.ValidateLocationAndPiece(d.Source, d.Piece, d.Token)
    if err != "" {
        g.subactionError(c, "Source Error", err)
        return false
    }
    err = g.table.ValidateLocation(d.Dest)
    if err != "" {
        g.subactionError(c, "Dest Error", err)
        return false
    }

    return true
}

// Either this mutates nothing and sends back a single NotifySubactionError to
// the player, or it mutates our state, steps turnState forward, and sends a
// single notifySubaction to everyone with the performed subaction and the
// updated turnState.
func (g *Game) handleDoSubaction(p int, c client.Client, d simple.Subaction) {
    g.debugf("Handle doSubaction: %d, %v", p, d)
    if !g.validateSubaction(p, c, d) {
        return
    }

    // If your turn is over.
    if g.turnState.Type == simple.NoneTurnStateType && g.turnState.ActionsLeft == 0 {
        g.subactionError(c, "Subaction Error", "You have no actions left")
        return
    }

    if d.Source.Type == simple.CityLocationType {
        g.subactionError(c, "Nope!", "You can not move pieces from an Office")
        return
    }

    if d.Source.Type == simple.PlayerLocationType {
        if d.Source.Id != p {
            g.subactionError(c, "Subaction Error", "You can not move pieces from other player boards")
            return
        }
        if d.Source.Index < 5 {
            if d.Dest.Type != simple.PlayerLocationType {
                g.subactionError(c, "Subaction Error", "You can only clear to your supply")
                return
            }
            if d.Dest.Id != p {
                g.subactionError(c, "Subaction Error", "You can only clear to your supply")
                return
            }
            if d.Dest.Index != 6 {
                g.subactionError(c, "Subaction Error", "You can only clear to your supply")
                return
            }
            if g.table.PlayerBoards[p].Supply[d.Dest.Subindex] != (simple.Piece{}) {
                g.subactionError(c, "Subaction Error", "There is already a piece in Dest")
                return
            }
            if g.turnState.Type != simple.Clearing {
                g.subactionError(c, "Subaction Error", "You can not level up unless you are clearing a route")
                return
            }
            if g.turnState.ClearingAward == simple.NoneAward ||
                g.turnState.ClearingAward == simple.CoellenAward{
                g.subactionError(c, "Subaction Error", "You have no clearing award for that track")
                return
            }
            if g.turnState.ClearingAward == simple.DiscsAward {
                if d.Source.Index != 3 {
                    g.subactionError(c, "Subaction Error", "Your clearing award is for the Books track")
                    return
                }
                if d.Source.Subindex != g.table.PlayerBoards[p].GetLeftmostBookDisc() {
                    g.subactionError(c, "Subaction Error", "You must remove the left most piece")
                    return
                }
            }
            if g.turnState.ClearingAward == simple.PriviledgeAward {
                if d.Source.Index != 2 {
                    g.subactionError(c, "Subaction Error", "Your clearing award is for the Priviledge track")
                    return
                }
                if d.Source.Subindex != g.table.PlayerBoards[p].GetLeftmostPriviledgeCube() {
                    g.subactionError(c, "Subaction Error", "You must remove the left most piece")
                    return
                }
            }
            if g.turnState.ClearingAward == simple.BagsAward {
                if d.Source.Index != 4 {
                    g.subactionError(c, "Subaction Error", "Your clearing award is for the Bags track")
                    return
                }
                if d.Source.Subindex != g.table.PlayerBoards[p].GetLeftmostBagCube() {
                    g.subactionError(c, "Subaction Error", "You must remove the left most piece")
                    return
                }
            }
            if g.turnState.ClearingAward == simple.ActionsAward {
                if d.Source.Index != 1 {
                    g.subactionError(c, "Subaction Error", "Your clearing award is for the Actions track")
                    return
                }
                if d.Source.Subindex != g.table.PlayerBoards[p].GetLeftmostActionCube() {
                    g.subactionError(c, "Subaction Error", "You must remove the left most piece")
                    return
                }
            }
            if g.turnState.ClearingAward == simple.KeysAward {
                if d.Source.Index != 0 {
                    g.subactionError(c, "Subaction Error", "Your clearing award is for the Keys track")
                    return
                }
                if d.Source.Subindex != g.table.PlayerBoards[p].GetLeftmostKeyCube() {
                    g.subactionError(c, "Subaction Error", "You must remove the left most piece")
                    return
                }
            }

            g.turnState.ClearingAward = simple.NoneAward
            g.turnState.ClearingCanOffice = false

            startActionsBefore := g.table.PlayerBoards[p].GetActions()
            g.applySubaction(p, d)
            startActionsAfter := g.table.PlayerBoards[p].GetActions()
            if startActionsAfter > startActionsBefore {
                g.turnState.ActionsLeft++
            }

            left := false
            for _, piece := range g.table.Board.Routes[g.turnState.ClearingRouteId].Spots{
                if piece != (simple.Piece{}) {
                    left = true
                    break
                }
            }
            if !left {
                g.turnState.Type = simple.NoneTurnStateType
                g.turnState.ClearingRouteId = 0
                g.actions = append(g.actions, simple.Action{
                    Type: simple.ClearActionType,
                    Subactions: g.subactions,
                })
                g.subactions = []simple.Subaction{}
            }
            g.gameEndIfNecessary()
            g.notifySubaction(d)
            return
        }
        if d.Source.Index == 5 {
            if d.Dest.Type == simple.CityLocationType {
                g.subactionError(c, "Subaction Error", "You can not move pieces from Stock to an Office")
                return
            }
            if d.Dest.Type == simple.RouteLocationType {
                if g.turnState.Type != simple.Bumping {
                    g.subactionError(c, "Subaction Error", "You can only move from Stock to Route when bumped")
                    return
                }
                if g.table.Board.Routes[d.Dest.Id].Spots[d.Dest.Index] != (simple.Piece{}) {
                    g.subactionError(c, "Subaction Error", "You not bump when resolving a bump")
                    return
                }
                if d.Dest.Subindex != 0 {
                    g.subactionError(c, "Subaction Error", "You can not replace a bump to a bump zone")
                    return
                }
                if g.turnState.BumpingReplaces == 0 {
                    g.subactionError(c, "Subaction Error", "No bump replaces left (end bump)")
                    return
                }
                valid := g.table.ValidBumps(g.turnState.BumpingLocation)
                if !containsLocation(d.Dest, valid) {
                    g.subactionError(c, "Subaction Error",
                        "Route too far for bump replacement (there are %d valid  spots)", len(valid))
                    return
                }

                // Note we leave us in turnstate Bumping until EndBump is used explicitly.
                g.turnState.BumpingReplaces--
                g.applySubaction(p, d)
                g.gameEndIfNecessary()
                g.notifySubaction(d)
                return
            }

            // d.Dest.Type == simple.PlayerLocationType
            if d.Dest.Id != p {
                g.subactionError(c, "Subaction Error", "You can not move to another player board")
                return
            }
            if d.Dest.Index != 6 {
                g.subactionError(c, "Subaction Error", "You can only move pieces from Stock to Supply")
                return
            }
            if g.table.PlayerBoards[p].Supply[d.Dest.Subindex] != (simple.Piece{}) {
                g.subactionError(c, "Subaction Error", "There is already a piece in that Subindex")
                return
            }
            if g.turnState.Type == simple.BumpPaying {
                g.subactionError(c, "Subaction Error", "You must complete bump payment (drag from supply to stock)")
                return
            }
            if g.turnState.Type == simple.Bumping {
                g.subactionError(c, "Subaction Error", "You can not bags while resolving a bump")
                return
            }
            if g.turnState.Type == simple.Clearing {
                g.subactionError(c, "Subaction Error", "You can not bags while clearing a route")
                return
            }

            if g.turnState.Type == simple.Moving {
                if g.turnState.ActionsLeft == 0 {
                    g.subactionError(c, "Subaction Error", "You have no actions after this Move action")
                    return
                }
                if g.gameend {
                    g.subactionError(c, "Subaction Error", "The game is ending after this Move action")
                    return
                }
                g.actions = append(g.actions, simple.Action{
                    Type: simple.MoveActionType,
                    Subactions: g.subactions,
                })
                g.subactions = []simple.Subaction{}
                g.turnState.MovesLeft = 0
                g.turnState.Type = simple.Bags
                g.turnState.ActionsLeft--
                g.turnState.BagsLeft = g.table.PlayerBoards[p].GetBags()-1
                g.applySubaction(p, d)
                g.gameEndIfNecessary()
                g.notifySubaction(d)

            } else if g.turnState.Type == simple.Bags {
                g.turnState.BagsLeft--
                g.applySubaction(p, d)
                if g.turnState.BagsLeft == 0 {
                    g.turnState.Type = simple.NoneTurnStateType
                    g.actions = append(g.actions, simple.Action{
                        Type: simple.BagsActionType,
                        Subactions: g.subactions,
                    })
                    g.subactions = []simple.Subaction{}
                }
                g.gameEndIfNecessary()
                g.notifySubaction(d)

            } else if g.turnState.Type == simple.NoneTurnStateType {
                g.turnState.Type = simple.Bags
                g.turnState.ActionsLeft--
                g.turnState.BagsLeft = g.table.PlayerBoards[p].GetBags()-1
                g.applySubaction(p, d)
                g.gameEndIfNecessary()
                g.notifySubaction(d)
            }

            return
        }
        if d.Source.Index == 6 {
            if d.Dest.Type == simple.CityLocationType {
                g.subactionError(c, "Subaction Error", "You can only move pieces from Supply Route or Stock")
                return
            }
            if d.Dest.Type == simple.PlayerLocationType {
                if d.Dest.Id != d.Source.Id {
                    g.subactionError(c, "Subaction Error", "You can not move pieces to another player board")
                    return
                }
                if d.Dest.Index < 5 {
                    g.subactionError(c, "Subaction Error", "You can't move on to the level up tracks")
                    return
                }
                if d.Dest.Index == 6 {
                    g.subactionError(c, "Subaction Error", "You can't move pieces within your supply")
                    return
                }
                if d.Dest.Index > 6 {
                    g.subactionError(c, "Subaction Error", "You can't move non tokens here")
                    return
                }
                if g.table.PlayerBoards[p].Stock[d.Dest.Subindex] != (simple.Piece{}) {
                    g.subactionError(c, "Subaction Error", "There is already a piece in Dest")
                    return
                }
                if g.turnState.Type != simple.BumpPaying {
                    g.subactionError(c, "Subaction Error", "You can only move Supply to Stock during bump pay")
                    return
                }

                g.turnState.BumpPayingCost--
                g.applySubaction(p, d)
                if g.turnState.BumpPayingCost == 0 {
                    g.turnState.Type = simple.Bumping
                    g.turnState.BumpingStart = time.Now()
                }
                g.gameEndIfNecessary()
                g.notifySubaction(d)
                return
            }
            if d.Dest.Type == simple.RouteLocationType {
                if d.Dest.Subindex != 0 {
                    g.subactionError(c, "Subaction Error", "You can not move a piece to the bumped zone")
                    return
                }
                bump := g.table.Board.Routes[d.Dest.Id].Spots[d.Dest.Index]
                if g.colorToPlayer(bump.PlayerColor) == p {
                    g.subactionError(c, "Subaction Error", "You can not bump yourself")
                    return
                }

                if g.turnState.Type == simple.BumpPaying {
                    g.subactionError(c, "Subaction Error", "You must pay for your bump")
                    return
                }
                if g.turnState.Type == simple.Clearing {
                    g.subactionError(c, "Subaction Error", "You can not place new pieces while clearing route")
                    return
                }

                if g.turnState.Type == simple.Bumping {
                    if bump != (simple.Piece{}) {
                        g.subactionError(c, "Subaction Error", "You can not bump when resolving a bump")
                        return
                    }
                    if g.turnState.BumpingReplaces == 0 {
                        g.subactionError(c, "Subaction Error", "No bump replaces left (end bump)")
                        return
                    }
                    for _, piece := range g.table.PlayerBoards[p].Stock {
                        if piece != (simple.Piece{}) {
                            g.subactionError(c, "Subaction Error",
                                "Can not replace from supply when there are pieces in stock.")
                            return
                        }
                    }
                    valid := g.table.ValidBumps(g.turnState.BumpingLocation)
                    if !containsLocation(d.Dest, valid) {
                        g.subactionError(c, "Subaction Error",
                            "Route too far for bump replacement (there are %d valid  spots)", len(valid))
                        return
                    }

                    // Note we leave us in turnstate Bumping until EndBump is used explicitly.
                    g.turnState.BumpingReplaces--
                    g.applySubaction(p, d)
                    g.gameEndIfNecessary()
                    g.notifySubaction(d)
                    return
                }

                if bump.PlayerColor != simple.NonePlayerColor {
                    need := 2
                    if bump.Shape == simple.DiscShape {
                        need = 3
                    }
                    have := 0
                    for _, supplyP := range g.table.PlayerBoards[p].Supply {
                        if supplyP != (simple.Piece{}) {
                            have++
                        }
                    }
                    if have < need {
                        g.subactionError(c, "Subaction Error",
                            "You can not afford that bump (need %d have %d)", need, have)
                        return 
                    }
                }

                if g.turnState.Type == simple.Moving {
                    if g.turnState.ActionsLeft == 0 {
                        g.subactionError(c, "Subaction Error", "You have no actions after this Move action")
                        return
                    }
                    if g.gameend {
                        g.subactionError(c, "Subaction Error", "The game is ending after this Move action")
                        return
                    }
                    g.actions = append(g.actions, simple.Action{
                        Type: simple.MoveActionType,
                        Subactions: g.subactions,
                    })
                    g.subactions = []simple.Subaction{}
                    g.turnState.MovesLeft = 0
                    g.turnState.ActionsLeft--
                    if bump.PlayerColor == simple.NonePlayerColor {
                        g.turnState.Type = simple.NoneTurnStateType
                        g.applySubaction(p, d)
                        g.actions = append(g.actions, simple.Action{
                            Type: simple.PlaceActionType,
                            Subactions: g.subactions,
                        })
                        g.subactions = []simple.Subaction{}
                    } else {
                        g.turnState.Type = simple.BumpPaying
                        g.turnState.BumpPayingCost = 1
                        g.turnState.BumpingPlayer = g.colorToPlayer(bump.PlayerColor)
                        g.turnState.BumpingLocation = simple.Location{
                            Type: d.Dest.Type,
                            Id: d.Dest.Id,
                            Index: d.Dest.Index,
                            Subindex: 1,
                        }
                        g.turnState.BumpingMoved = false
                        g.turnState.BumpingReplaces = 1
                        if bump.Shape == simple.DiscShape {
                            g.turnState.BumpPayingCost = 2
                            g.turnState.BumpingReplaces = 2
                        }
                        g.applySubaction(p, d)
                    }
                    g.gameEndIfNecessary()
                    g.notifySubaction(d)

                } else if g.turnState.Type == simple.Bags {
                    if g.turnState.ActionsLeft == 0 {
                        g.subactionError(c, "Subaction Error", "You have no actions after this Bags action")
                        return
                    }
                    if g.gameend {
                        g.subactionError(c, "Subaction Error", "The game is ending after this Bags action")
                        return
                    }
                    g.actions = append(g.actions, simple.Action{
                        Type: simple.BagsActionType,
                        Subactions: g.subactions,
                    })
                    g.subactions = []simple.Subaction{}
                    g.turnState.BagsLeft = 0
                    g.turnState.ActionsLeft--

                    if bump.PlayerColor == simple.NonePlayerColor {
                        g.turnState.Type = simple.NoneTurnStateType
                        g.applySubaction(p, d)
                        g.actions = append(g.actions, simple.Action{
                            Type: simple.PlaceActionType,
                            Subactions: g.subactions,
                        })
                        g.subactions = []simple.Subaction{}
                    } else {
                        g.turnState.Type = simple.BumpPaying
                        g.turnState.BumpPayingCost = 1
                        g.turnState.BumpingPlayer = g.colorToPlayer(bump.PlayerColor)
                        g.turnState.BumpingLocation = simple.Location{
                            Type: d.Dest.Type,
                            Id: d.Dest.Id,
                            Index: d.Dest.Index,
                            Subindex: 1,
                        }
                        g.turnState.BumpingMoved = false
                        g.turnState.BumpingReplaces = 1
                        if bump.Shape == simple.DiscShape {
                            g.turnState.BumpPayingCost = 2
                            g.turnState.BumpingReplaces = 2
                        }
                        g.applySubaction(p, d)
                    }
                    g.gameEndIfNecessary()
                    g.notifySubaction(d)

                } else if g.turnState.Type == simple.NoneTurnStateType {
                    g.turnState.ActionsLeft--
                    if bump.PlayerColor == simple.NonePlayerColor {
                        g.turnState.Type = simple.NoneTurnStateType
                        g.applySubaction(p, d)
                        g.actions = append(g.actions, simple.Action{
                            Type: simple.PlaceActionType,
                            Subactions: g.subactions,
                        })
                        g.subactions = []simple.Subaction{}
                    } else {
                        g.turnState.Type = simple.BumpPaying
                        g.turnState.BumpPayingCost = 1
                        g.turnState.BumpingPlayer = g.colorToPlayer(bump.PlayerColor)
                        g.turnState.BumpingLocation = simple.Location{
                            Type: d.Dest.Type,
                            Id: d.Dest.Id,
                            Index: d.Dest.Index,
                            Subindex: 1,
                        }
                        g.turnState.BumpingMoved = false
                        g.turnState.BumpingReplaces = 1
                        if bump.Shape == simple.DiscShape {
                            g.turnState.BumpPayingCost = 2
                            g.turnState.BumpingReplaces = 2
                        }
                        g.applySubaction(p, d)
                    }
                    g.gameEndIfNecessary()
                    g.notifySubaction(d)
                }
                return
            }
        }
    }

    if d.Source.Type == simple.RouteLocationType {
        if d.Piece == (simple.Piece{}) {
            g.subactionError(c, "Subaction Error", "You can not move tokens from routes")
            return
        }
        if d.Piece.PlayerColor != g.table.PlayerBoards[p].Color {
            g.subactionError(c, "Subaction Error", "You can only move your own pieces from routes")
            return
        }
        if d.Dest.Type == simple.RouteLocationType {
            if d.Dest.Subindex != 0 {
                g.subactionError(c, "Subaction Error", "You can not move a piece to the bumped zone")
                return
            }
            if g.table.Board.Routes[d.Dest.Id].Spots[d.Dest.Index] != (simple.Piece{}) {
                g.subactionError(c, "Subaction Error", "You can not bump while moving")
                return
            }

            if g.turnState.Type == simple.BumpPaying {
                g.subactionError(c, "Subaction Error", "You must pay for your bump")
                return
            }
            if g.turnState.Type == simple.Clearing {
                g.subactionError(c, "Subaction Error", "You can not move while clearing")
                return
            }

            if g.turnState.Type == simple.Bumping {
                valid := g.table.ValidBumps(g.turnState.BumpingLocation)
                if !containsLocation(d.Dest, valid) {
                    g.subactionError(c, "Subaction Error",
                        "Route too far for bump replacement (there are %d valid  spots)", len(valid))
                    return
                }
                if d.Source.Subindex == 1 {
                    if g.turnState.BumpingMoved {
                        g.subactionError(c, "Subaction Error", "You have already moved your bumped piece")
                        return
                    }

                    g.turnState.BumpingMoved = true
                    g.applySubaction(p, d)
                    g.gameEndIfNecessary()
                    g.notifySubaction(d)
                    return
                }

                // We can only replace from the board if we have an empty stock and supply.
                if g.turnState.BumpingReplaces == 0 {
                    g.subactionError(c, "Subaction Error", "No bump replaces left (end bump)")
                    return
                }
                for _, piece := range g.table.PlayerBoards[p].Stock {
                    if piece != (simple.Piece{}) {
                        g.subactionError(c, "Subaction Error",
                            "Can not replace from the board when there are pieces in stock.")
                        return
                    }
                }
                for _, piece := range g.table.PlayerBoards[p].Supply {
                    if piece != (simple.Piece{}) {
                        g.subactionError(c, "Subaction Error",
                            "Can not replace from the board when there are pieces in supply.")
                        return
                    }
                }

                // Note we leave us in turnstate Bumping until EndBump is used explicitly.
                g.turnState.BumpingReplaces--
                g.applySubaction(p, d)
                g.gameEndIfNecessary()
                g.notifySubaction(d)
                return
            }

            if g.turnState.Type == simple.Moving {
                g.turnState.MovesLeft--
                g.applySubaction(p, d)
                if g.turnState.MovesLeft == 0 {
                    g.turnState.Type = simple.NoneTurnStateType
                    g.actions = append(g.actions, simple.Action{
                        Type: simple.MoveActionType,
                        Subactions: g.subactions,
                    })
                    g.subactions = []simple.Subaction{}
                }
                g.gameEndIfNecessary()
                g.notifySubaction(d)

            } else if g.turnState.Type == simple.Bags {
                if g.turnState.ActionsLeft == 0 {
                    g.subactionError(c, "Subaction Error", "You have no actions after this Bag action")
                    return
                }
                if g.gameend {
                    g.subactionError(c, "Subaction Error", "The game is ending after this Bags action")
                    return
                }
                g.actions = append(g.actions, simple.Action{
                    Type: simple.BagsActionType,
                    Subactions: g.subactions,
                })
                g.subactions = []simple.Subaction{}
                g.turnState.BagsLeft = 0
                g.turnState.Type = simple.Moving
                g.turnState.ActionsLeft--
                g.turnState.MovesLeft = g.table.PlayerBoards[p].GetBooks()-1
                g.applySubaction(p, d)
                g.gameEndIfNecessary()
                g.notifySubaction(d)

            } else if g.turnState.Type == simple.NoneTurnStateType {
                g.turnState.Type = simple.Moving
                g.turnState.ActionsLeft--
                g.turnState.MovesLeft = g.table.PlayerBoards[p].GetBooks()-1
                g.applySubaction(p, d)
                g.gameEndIfNecessary()
                g.notifySubaction(d)
            }

            return
        }
        if d.Dest.Type == simple.CityLocationType {
            if d.Dest.Subindex != 0 && d.Dest.Subindex != 2 {
                g.subactionError(c, "Subaction Error", "You can not use virtual offices (yet)")
                return
            }
            if d.Dest.Subindex == 2 {
                if g.turnState.Type != simple.Clearing {
                    g.subactionError(c, "Subaction Error", "Begin clearing the route to take a reward")
                    return
                }
                if g.turnState.ClearingAward != simple.CoellenAward {
                    g.subactionError(c, "Subaction Error", "You don't have the Coellen reward")
                    return
                }
                if d.Source.Id != g.turnState.ClearingRouteId {
                    g.subactionError(c, "Subaction Error", "Coellen piece must come from the cleared route")
                    return
                }
                if d.Piece.Shape != simple.DiscShape {
                    g.subactionError(c, "Subaction Error", "Coellen piece must be a disc")
                    return
                }
                spot := g.table.Board.Cities[d.Dest.Id].Coellen.Spots[d.Dest.Index]
                if spot.Piece != (simple.Piece{}) {
                    g.subactionError(c, "Subaction Error", "There is already a piece in Dest")
                    return
                }
                if spot.Priviledge > g.table.PlayerBoards[p].GetPriviledge() {
                    g.subactionError(c, "Subaction Error", "You do not have the priviledge for that spot")
                    return
                }

                g.turnState.ClearingAward = simple.NoneAward
                g.turnState.ClearingCanOffice = false
                g.applySubaction(p, d)
                left := false
                for _, piece := range g.table.Board.Routes[g.turnState.ClearingRouteId].Spots{
                    if piece != (simple.Piece{}) {
                        left = true
                        break
                    }
                }
                if !left {
                    g.turnState.Type = simple.NoneTurnStateType
                    g.turnState.ClearingRouteId = 0
                    g.actions = append(g.actions, simple.Action{
                        Type: simple.ClearActionType,
                        Subactions: g.subactions,
                    })
                    g.subactions = []simple.Subaction{}
                }
                g.gameEndIfNecessary()
                g.notifySubaction(d)
                return
            }

            office := g.table.Board.Cities[d.Dest.Id].Offices[d.Dest.Index]
            if office.Piece != (simple.Piece{}) {
                g.subactionError(c, "Subaction Error", "Office is not empty")
                return
            }
            for i, o := range g.table.Board.Cities[d.Dest.Id].Offices {
                if i == d.Dest.Index {
                    break
                }
                if o.Piece == (simple.Piece{}) {
                    g.subactionError(c, "Subaction Error", "Must take leftmost open office")
                    return
                }
            }
            if office.Shape != d.Piece.Shape {
                g.subactionError(c, "Subaction Error", "Piece does not fit in that Office Shape")
                return
            }
            if g.table.PlayerBoards[p].GetPriviledge() < office.Priviledge {
                g.subactionError(c, "Subaction Error", "You lack the priviledge for that office")
                return
            }
            if office.Shape != d.Piece.Shape {
                g.subactionError(c, "Subaction Error", "Piece does not fit in that Office Shape")
                return
            }
            if g.turnState.Type == simple.BumpPaying {
                g.subactionError(c, "Subaction Error", "Finish paying for your bump")
                return
            }
            if g.turnState.Type == simple.Bumping {
                g.subactionError(c, "Subaction Error", "You can not be bumped into an office")
                return
            }

            if g.turnState.Type == simple.Clearing {
                if !g.turnState.ClearingCanOffice {
                    g.subactionError(c, "Subaction Error", "You have already taken a clearing reward")
                    return
                }
                route := g.table.Board.Routes[g.turnState.ClearingRouteId]
                if route.LeftCityId != d.Dest.Id && route.RightCityId != d.Dest.Id {
                    g.subactionError(c, "Subaction Error", "City is not adjacent to the cleared route")
                    return
                }
                if d.Source.Id != route.Id {
                    g.subactionError(c, "Subaction Error", "Office piece must come from cleared route")
                    return
                }

                g.turnState.ClearingCanOffice = false
                g.turnState.ClearingAward = simple.NoneAward
                scores := make([]int, len(g.table.PlayerBoards))
                scores[p] = office.Points
                g.applySubaction(p, d)
                scores[p]+= g.bonusRouteScoreIfNecessary(p)
                left := false
                for _, piece := range g.table.Board.Routes[g.turnState.ClearingRouteId].Spots{ 
                    if piece != (simple.Piece{}) {
                        left = true
                        break
                    }
                }
                if !left {
                    g.turnState.Type = simple.NoneTurnStateType
                    g.turnState.ClearingRouteId = 0
                    g.actions = append(g.actions, simple.Action{
                        Type: simple.ClearActionType,
                        Subactions: g.subactions,
                    })
                    g.subactions = []simple.Subaction{}
                }
                g.updateScores(scores)
                g.gameEndIfNecessary()
                g.notifySubactionWithScores(d, scores)
                return
            }
            for _, piece := range g.table.Board.Routes[d.Source.Id].Spots {
                if piece.PlayerColor != d.Piece.PlayerColor {
                    g.subactionError(c, "Subaction Error", "You can't clear a non full route")
                    return
                }
            }
            if g.turnState.Type == simple.Moving {
                if g.turnState.ActionsLeft == 0 {
                    g.subactionError(c, "Subaction Error", "You have no actions after this Move action")
                    return
                }
                if g.gameend {
                    g.subactionError(c, "Subaction Error", "The game is ending after this Move action")
                    return
                }
                g.actions = append(g.actions, simple.Action{
                    Type: simple.MoveActionType,
                    Subactions: g.subactions,
                })
                g.subactions = []simple.Subaction{}
                g.turnState.MovesLeft = 0
                g.turnState.ActionsLeft--
                g.turnState.Type = simple.Clearing
                g.turnState.ClearingRouteId = d.Source.Id
                g.turnState.ClearingCanOffice = false
                g.turnState.ClearingAward = simple.NoneAward
                scores := make([]int, len(g.table.PlayerBoards))
                scores[p] += office.Points
                controlL := g.table.Board.Cities[g.table.Board.Routes[d.Source.Id].LeftCityId].GetControl()
                controlR := g.table.Board.Cities[g.table.Board.Routes[d.Source.Id].RightCityId].GetControl()
                if controlL != simple.NonePlayerColor {
                    scores[g.colorToPlayer(controlL)] += 1
                }
                if controlR != simple.NonePlayerColor {
                    scores[g.colorToPlayer(controlR)] += 1
                }
                g.applySubaction(p, d)
                scores[p]+= g.bonusRouteScoreIfNecessary(p)
                g.updateScores(scores)
                g.gameEndIfNecessary()
                g.notifySubactionWithScores(d, scores)
                return
            }
            if g.turnState.Type == simple.Bags {
                if g.turnState.ActionsLeft == 0 {
                    g.subactionError(c, "Subaction Error", "You have no actions after this Bags action")
                    return
                }
                if g.gameend {
                    g.subactionError(c, "Subaction Error", "The game is ending after this Bags action")
                    return
                }
                g.actions = append(g.actions, simple.Action{
                    Type: simple.MoveActionType,
                    Subactions: g.subactions,
                })
                g.subactions = []simple.Subaction{}
                g.turnState.BagsLeft = 0
                g.turnState.ActionsLeft--
                g.turnState.Type = simple.Clearing
                g.turnState.ClearingRouteId = d.Source.Id
                g.turnState.ClearingCanOffice = false
                g.turnState.ClearingAward = simple.NoneAward
                scores := make([]int, len(g.table.PlayerBoards))
                scores[p] += office.Points
                controlL := g.table.Board.Cities[g.table.Board.Routes[d.Source.Id].LeftCityId].GetControl()
                controlR := g.table.Board.Cities[g.table.Board.Routes[d.Source.Id].RightCityId].GetControl()
                if controlL != simple.NonePlayerColor {
                    scores[g.colorToPlayer(controlL)] += 1
                }
                if controlR != simple.NonePlayerColor {
                    scores[g.colorToPlayer(controlR)] += 1
                }
                g.applySubaction(p, d)
                scores[p]+= g.bonusRouteScoreIfNecessary(p)
                g.updateScores(scores)
                g.gameEndIfNecessary()
                g.notifySubactionWithScores(d, scores)
                return
            }
            if g.turnState.Type == simple.NoneTurnStateType {
                g.turnState.ActionsLeft--
                g.turnState.Type = simple.Clearing
                g.turnState.ClearingRouteId = d.Source.Id
                g.turnState.ClearingCanOffice = false
                g.turnState.ClearingAward = simple.NoneAward
                scores := make([]int, len(g.table.PlayerBoards))
                scores[p] += office.Points
                controlL := g.table.Board.Cities[g.table.Board.Routes[d.Source.Id].LeftCityId].GetControl()
                controlR := g.table.Board.Cities[g.table.Board.Routes[d.Source.Id].RightCityId].GetControl()
                if controlL != simple.NonePlayerColor {
                    scores[g.colorToPlayer(controlL)] += 1
                }
                if controlR != simple.NonePlayerColor {
                    scores[g.colorToPlayer(controlR)] += 1
                }
                g.applySubaction(p, d)
                scores[p]+= g.bonusRouteScoreIfNecessary(p)
                g.updateScores(scores)
                g.gameEndIfNecessary()
                g.notifySubactionWithScores(d, scores)
                return
            }
        }
        if d.Dest.Type == simple.PlayerLocationType {
            if d.Dest.Id != p {
                g.subactionError(c, "Subaction Error", "You can not clear to another player board")
                return
            }
            if d.Dest.Index != 5 {
                g.subactionError(c, "Subaction Error", "You can only clear from a route to stock")
                return
            }
            if g.table.PlayerBoards[p].Stock[d.Dest.Subindex] != (simple.Piece{}) {
                g.subactionError(c, "Subaction Error", "There is already a piece in that subindex")
                return
            }
            if g.turnState.Type == simple.BumpPaying {
                g.subactionError(c, "Subaction Error", "Finish paying for your bump")
                return
            }
            if g.turnState.Type == simple.Bumping {
                g.subactionError(c, "Subaction Error", "You can not be bumped into an office")
                return
            }
            if g.turnState.Type == simple.Clearing {
                route := g.table.Board.Routes[g.turnState.ClearingRouteId]
                if d.Source.Id != route.Id {
                    g.subactionError(c, "Subaction Error", "Finish clearing the other route")
                    return
                }
                g.applySubaction(p, d)
                left := false
                for _, piece := range g.table.Board.Routes[g.turnState.ClearingRouteId].Spots{ 
                    if piece != (simple.Piece{}) {
                        left = true
                        break
                    }
                }
                if !left && (g.turnState.ClearingAward == simple.NoneAward || g.turnState.ClearingAward == simple.CoellenAward) {
                    g.turnState.Type = simple.NoneTurnStateType
                    g.turnState.ClearingRouteId = 0
                    g.turnState.ClearingCanOffice = false
                    g.actions = append(g.actions, simple.Action{
                        Type: simple.ClearActionType,
                        Subactions: g.subactions,
                    })
                    g.subactions = []simple.Subaction{}
                }
                g.gameEndIfNecessary()
                g.notifySubaction(d)
                return
            }
            discsOnRouteAfterThisSubaction := 0
            for i, piece := range g.table.Board.Routes[d.Source.Id].Spots {
                if piece.PlayerColor != d.Piece.PlayerColor {
                    g.subactionError(c, "Subaction Error", "You can't clear a non full route")
                    return
                }
                if piece.Shape == simple.DiscShape && i != d.Source.Index {
                    discsOnRouteAfterThisSubaction++
                }
            }
            if g.turnState.Type == simple.Moving {
                if g.turnState.ActionsLeft == 0 {
                    g.subactionError(c, "Subaction Error", "You have no actions after this Move action")
                    return
                }
                if g.gameend {
                    g.subactionError(c, "Subaction Error", "The game is ending after this Move action")
                    return
                }
                g.actions = append(g.actions, simple.Action{
                    Type: simple.MoveActionType,
                    Subactions: g.subactions,
                })
                g.subactions = []simple.Subaction{}
                g.turnState.MovesLeft = 0
                g.turnState.ActionsLeft--
                g.turnState.Type = simple.Clearing
                g.turnState.ClearingRouteId = d.Source.Id
                g.turnState.ClearingCanOffice = true
                scores := make([]int, len(g.table.PlayerBoards))
                controlL := g.table.Board.Cities[g.table.Board.Routes[d.Source.Id].LeftCityId].GetControl()
                controlR := g.table.Board.Cities[g.table.Board.Routes[d.Source.Id].RightCityId].GetControl()
                if controlL != simple.NonePlayerColor {
                    scores[g.colorToPlayer(controlL)] += 1
                }
                if controlR != simple.NonePlayerColor {
                    scores[g.colorToPlayer(controlR)] += 1
                }
                route := g.table.Board.Routes[d.Source.Id]
                award := g.table.Board.Cities[route.LeftCityId].Award
                if award == simple.NoneAward {
                    award = g.table.Board.Cities[route.RightCityId].Award
                }
                if (award == simple.CoellenAward && discsOnRouteAfterThisSubaction == 0) || 
                    (award != simple.CoellenAward && !g.table.PlayerBoards[p].CanAward(award)) {
                    award = simple.NoneAward
                }
                g.turnState.ClearingAward = award
                g.applySubaction(p, d)
                g.updateScores(scores)
                g.gameEndIfNecessary()
                g.notifySubactionWithScores(d, scores)
                return
            }
            if g.turnState.Type == simple.Bags {
                if g.turnState.ActionsLeft == 0 {
                    g.subactionError(c, "Subaction Error", "You have no actions after this Bags action")
                    return
                }
                if g.gameend {
                    g.subactionError(c, "Subaction Error", "The game is ending after this Bags action")
                    return
                }
                g.actions = append(g.actions, simple.Action{
                    Type: simple.MoveActionType,
                    Subactions: g.subactions,
                })
                g.subactions = []simple.Subaction{}
                g.turnState.BagsLeft = 0
                g.turnState.ActionsLeft--
                g.turnState.Type = simple.Clearing
                g.turnState.ClearingRouteId = d.Source.Id
                g.turnState.ClearingCanOffice = true
                scores := make([]int, len(g.table.PlayerBoards))
                controlL := g.table.Board.Cities[g.table.Board.Routes[d.Source.Id].LeftCityId].GetControl()
                controlR := g.table.Board.Cities[g.table.Board.Routes[d.Source.Id].RightCityId].GetControl()
                if controlL != simple.NonePlayerColor {
                    scores[g.colorToPlayer(controlL)] += 1
                }
                if controlR != simple.NonePlayerColor {
                    scores[g.colorToPlayer(controlR)] += 1
                }
                route := g.table.Board.Routes[d.Source.Id]
                award := g.table.Board.Cities[route.LeftCityId].Award
                if award == simple.NoneAward {
                    award = g.table.Board.Cities[route.RightCityId].Award
                }
                if (award == simple.CoellenAward && discsOnRouteAfterThisSubaction == 0) || 
                    (award != simple.CoellenAward && !g.table.PlayerBoards[p].CanAward(award)) {
                    award = simple.NoneAward
                }
                g.turnState.ClearingAward = award
                g.applySubaction(p, d)
                g.updateScores(scores)
                g.gameEndIfNecessary()
                g.notifySubactionWithScores(d, scores)
                return
            }
            if g.turnState.Type == simple.NoneTurnStateType {
                g.turnState.ActionsLeft--
                g.turnState.Type = simple.Clearing
                g.turnState.ClearingRouteId = d.Source.Id
                g.turnState.ClearingCanOffice = true
                scores := make([]int, len(g.table.PlayerBoards))
                controlL := g.table.Board.Cities[g.table.Board.Routes[d.Source.Id].LeftCityId].GetControl()
                controlR := g.table.Board.Cities[g.table.Board.Routes[d.Source.Id].RightCityId].GetControl()
                if controlL != simple.NonePlayerColor {
                    scores[g.colorToPlayer(controlL)] += 1
                }
                if controlR != simple.NonePlayerColor {
                    scores[g.colorToPlayer(controlR)] += 1
                }
                route := g.table.Board.Routes[d.Source.Id]
                award := g.table.Board.Cities[route.LeftCityId].Award
                if award == simple.NoneAward {
                    award = g.table.Board.Cities[route.RightCityId].Award
                }
                if (award == simple.CoellenAward && discsOnRouteAfterThisSubaction == 0) || 
                    (award != simple.CoellenAward && !g.table.PlayerBoards[p].CanAward(award)) {
                    award = simple.NoneAward
                }
                g.turnState.ClearingAward = award
                g.applySubaction(p, d)
                g.updateScores(scores)
                g.gameEndIfNecessary()
                g.notifySubactionWithScores(d, scores)
                return
            }
        }
    }
}

func (g *Game) bonusRouteScoreIfNecessary(p int) int {
    worth := 7
    for i, b := range g.bonusroute {
        if b {
            if i == p {
                return 0
            } else if worth == 7 {
                worth = 4
            } else if worth == 4 {
                worth = 2
            } else {
                return 0
            }
        }
    }

    if g.table.Board.GetBonusRouteCompleted(g.table.PlayerBoards[p].Color) {
        g.bonusroute[p] = true
        return worth
    }
    return 0
}

func (g *Game) handleEndTurn(p int, c client.Client, d message.EndTurnData) {
    if g.turnState.Player != p {
        g.clientError(c, "Endturn Error", "It's not your turn")
        return
    }
    if g.turnState.Type == simple.Bumping {
        g.clientError(c, "Endturn Error", "Wait for opponent to react to the bump")
        return
    }
    if g.turnState.Type == simple.Clearing {
        g.clientError(c, "Endturn Error", "You must complete clearing the route before ending your turn (including rewards)")
        return
    }

    g.debugf("Endturn (Player %d)", p)
    g.times.elapsed[g.turnState.Player] +=
        time.Now().Sub(g.turnState.TurnStart) +
        time.Duration(g.turnState.TurnElapsedDelta)

    if g.gameend {
        g.newStatus = Scoring
        return
    }

    next := (g.turnState.Player + 1) % len(g.players)
    g.turnState = simple.TurnState{
        Type: simple.NoneTurnStateType,
        Player: next,
        ActionsLeft: g.table.PlayerBoards[next].GetActions(),
        TurnStart: time.Now(),
    }

    msg := message.Server{
        SType: message.NotifyNextTurn,
        Time: time.Now(),
        Data: message.NotifyNextTurnData{
            TurnState: g.turnState,
            Elapsed: g.castElapsed(),
        },
    }
    g.debugf("NextTurn (Player %d)", g.turnState.Player)
    g.notify(msg)
}

func (g *Game) handleEndBump(p int, c client.Client, d message.EndBumpData) {
    if g.turnState.Type != simple.Bumping {
        g.clientError(c, "Endturn Error", "There is no bump in progress")
        return
    }
    if g.turnState.BumpingPlayer != p {
        g.clientError(c, "Endturn Error", "You are not being bumped")
        return
    }
    if !g.turnState.BumpingMoved {
        g.clientError(c, "Endturn Error", "You must move your bumped piece to another the route")
        return
    }

    g.debugf("Endbump (Player %d)", p)
    g.times.elapsed[g.turnState.BumpingPlayer] += time.Now().Sub(g.turnState.BumpingStart)
    g.turnState.Type = simple.NoneTurnStateType
    g.turnState.BumpingPlayer = 0 // ew
    g.turnState.BumpingLocation = simple.Location{}
    g.turnState.BumpingMoved = false
    g.turnState.BumpingReplaces = 0

    msg := message.Server{
        SType: message.NotifyEndBump,
        Time: time.Now(),
        Data: message.NotifyEndBumpData{
            TurnState: g.turnState,
            Elapsed: g.castElapsed(),
        },
    }
    g.notify(msg)
}

// This validates nothing, and mutates our state.  This is done last after all
// validation is complete.  This only mutates the Board and Player.Board by
// moving pieces and tokens.  If dest is occupied, it is moved to source.  If
// dest is a route, it is instead moved to bumped.  If dest is supply, source
// is a route, and bumped is occupied, bumped is moved to source.
func (g *Game) applySubaction(p int, s simple.Subaction) {
    g.debugf("Subaction (Player %d): %v", p, s)
    g.table.ApplySubaction(s, simple.EmptyIdentity)
    g.subactions = append(g.subactions, s)
}

func (g *Game) notifySubaction(s simple.Subaction) {
    ss := []int{}
    for i:=0;i<len(g.table.PlayerBoards);i++ {
        ss = append(ss, 0)
    }
    g.notifySubactionWithScores(s, ss)
}

func (g *Game) notifySubactionWithScores(s simple.Subaction, ss []int) {
    g.notify(message.Server{
        SType: message.NotifySubaction,
        Time: time.Now(),
        Data: message.NotifySubactionData{
            Subaction: s, 
            Scores: ss,
            TurnState: g.turnState,
            Gameend: g.gameend,
        },
    })
}

func (g *Game) checkStatus() {
    if g.status == g.newStatus {
        return
    }

    if g.status == Creating && g.newStatus == Running {

        // Place start tokens
        st := simple.NewBaseStartTokens()
        rand.Shuffle(len(st), func(i, j int) { st[i], st[j] = st[j], st[i] })
        for i, route := range g.table.Board.Routes {
            if route.StartToken {
                g.table.Board.Routes[i].Token = st[0]
                st = st[1:]
            }
        }

        // Remove empty player boards
        newPb := []simple.PlayerBoard{}
        for _, pb := range g.table.PlayerBoards {
            if pb.Identity != simple.EmptyIdentity {
                newPb = append(newPb, pb)
            }
        }
        g.table.PlayerBoards = newPb

        // Randomize player order ([0] is start player)
        rand.Shuffle(len(g.table.PlayerBoards), func(i, j int) {
            g.table.PlayerBoards[i], g.table.PlayerBoards[j] = g.table.PlayerBoards[j], g.table.PlayerBoards[i]
        })

        // Create clients for each player
        for _, pb := range g.table.PlayerBoards {
            var playerClient client.Client
            for i, c := range g.observers {
                if pb.Identity == i {
                    playerClient = c
                    delete(g.observers, i)
                    break
                }
            }
            if pb.Identity.Type == simple.IdentityTypeBot {
                playerClient = g.bm.NewBot(pb.Identity, g.Id)
            }
            if playerClient == nil {
                playerClient = client.NewDisconnectedMultiWebClient(pb.Identity)
                go playerClient.Run()
            }

            g.players = append(g.players, &Player{
                Client: playerClient,
            })
        }

        g.times.elapsed = []time.Duration{}
        g.scores = []int{}
        g.bonusroute = []bool{}
        for i, _ := range g.table.PlayerBoards {
            color := g.table.PlayerBoards[i].Color
            g.table.PlayerBoards[i].Supply[0] = simple.Piece{color, simple.DiscShape}
            for i2:=0;i2<11;i2++ {
                if i2 < 5+i {
                    g.table.PlayerBoards[i].Supply[i2+1] = simple.Piece{color, simple.CubeShape}
                } else {
                    g.table.PlayerBoards[i].Stock[i2-(5+i)] = simple.Piece{color, simple.CubeShape}
                }
            }
            g.times.elapsed = append(g.times.elapsed, time.Duration(0))
            g.scores = append(g.scores, 0)
            g.bonusroute = append(g.bonusroute, false)
        }
        g.times.running = time.Now()

        g.notify(message.Server{
            SType: message.NotifyStartGame,
            Time: time.Now(),
            Data: message.NotifyStartGameData{
                Table: *g.table,
            },
        })

        g.turnState = simple.TurnState{
            Type: simple.NoneTurnStateType,
            Player: 0,
            ActionsLeft: 2,
            TurnStart: time.Now(),
        }

        msg := message.Server{
            SType: message.NotifyNextTurn,
            Time: time.Now(),
            Data: message.NotifyNextTurnData{
                TurnState: g.turnState,
                Elapsed: g.castElapsed(),
            },
        }
        g.notify(msg)
    }

    if g.status == Running && g.newStatus == Scoring {
        g.finalscores = []map[simple.ScoreType]int{}
        localTotals := map[int]int{}
        for i:=0;i<len(g.table.PlayerBoards);i++ {
            g.finalscores = append(g.finalscores, map[simple.ScoreType]int{})
            localTotals[i] = 0
        }

        g.notify(message.Server{
            SType: message.NotifyScoringBegin,
            Time: time.Now(),
            Data: message.NotifyScoringBeginData{},
        })
        ms := []message.Server{}

        add := func(p int, t simple.ScoreType, s int) {
            localTotals[p] = localTotals[p] + s
            ms = append(ms, message.Server{
                SType: message.NotifyEndgameScoring,
                Time: time.Now(),
                Data: message.NotifyEndgameScoringData{
                    Player: p,
                    Type: t,
                    Score: s,
                },
            })
        }

        for i, s := range g.scores {
            add(i, simple.GameScoreType, s)
        }

        for i, pb := range g.table.PlayerBoards {
            s := 0
            if pb.GetActionCubes() == 0 {
                s+=4
            }
            if pb.GetBookDiscs() == 0 {
                s+=4
            }
            if pb.GetPriviledgeCubes() == 0 {
                s+=4
            }
            if pb.GetBagCubes() == 0 {
                s+=4
            }
            add(i, simple.BoardScoreType, s)
        }

        coellen := map[int]int{}
        control := map[int]int{}
        for i:=0;i<len(g.scores);i++ {
            coellen[i] = 0
            control[i] = 0
        }
        for _, c := range g.table.Board.Cities {
            controlC := c.GetControl()
            if controlC != simple.NonePlayerColor {
                p := g.colorToPlayer(controlC)
                control[p] = control[p] + 2
            }
            if c.Coellen.Spots != nil {
                for _, s := range c.Coellen.Spots {
                    if s.Piece != (simple.Piece{}) {
                        p := g.colorToPlayer(s.Piece.PlayerColor)
                        coellen[p] = coellen[p] + s.Points
                    }
                }
            }
        }
        for p, s := range coellen {
            add(p, simple.CoellenScoreType, s)
        }
        for p, s := range control {
            add(p, simple.ControlScoreType, s)
        }

        for i, pb := range g.table.PlayerBoards {
            keys := g.table.PlayerBoards[i].GetKeys()
            add(i, simple.NetworkScoreType, g.table.Board.GetNetworkScore(pb.Color) * keys)
        }

        localTotalsOrder := []int{}
        for p, s := range localTotals {
            add(p, simple.TotalScoreType, s)
            localTotalsOrder = append(localTotalsOrder, p)
        }
        sort.Slice(localTotalsOrder, func(i, j int) bool {
            return localTotals[localTotalsOrder[i]] < localTotals[localTotalsOrder[j]]
        })

        for i, p := range localTotalsOrder {
            add(p, simple.PlaceScoreType, len(localTotalsOrder)-1-i)
        }
        ms = append(ms, message.Server{
            SType: message.NotifyComplete,
            Time: time.Now(),
            Data: message.NotifyCompleteData{},
        })

        for i, m := range ms {
            innerCopy := m
            time.AfterFunc(
                time.Millisecond * time.Duration(500) * time.Duration(i+10), func() {
                g.scoring <- innerCopy
            })
        }
    }

    if g.status == Scoring && g.newStatus == Complete {

    }

    g.status = g.newStatus
}

func (g *Game) hotdeployLoad() {
    // TODO: this.
}

func (g *Game) updateScores(ss []int) {
    for i, s := range ss {
        g.scores[i] += s
    }
}

func (g *Game) gameEndIfNecessary() {
    if g.status != Running {
        return
    }
    if !g.gameend {
        end := false
        for _, s := range g.scores {
            // if s >= 1 {
            if s >= 20 {
                end = true
                break
            }
        }
        if !end {
            if g.table.Board.GetFilledCityCount() >= 10 {
                end = true
            }
        }
        if !end {
            return 
        }
        g.gameend = true
    }
    if g.turnState.Type != simple.NoneTurnStateType {
        return
    }
    g.turnState.ActionsLeft = 0
}

func (g *Game) colorToPlayer(c simple.PlayerColor) int {
    for i, pb := range g.table.PlayerBoards {
        if pb.Color == c {
            return i
        }
    }
    return -1
}

func (g *Game) panicking() {
    if r := recover(); r != nil {
        log.Stop(fmt.Sprintf("game %d panic", g.Id), r)
        panic(r)
    }
}

func (g *Game) clientError(c client.Client, header string, content string, fargs ...interface{}) {
    content = fmt.Sprintf(content, fargs...)
    g.debugf("(ClientError) (%s) %s: %s", c.Identity(), header, content)
    c.Send(message.NewNotifyNotification(message.NotificationError, header, content))
}

func (g *Game) subactionError(c client.Client, header string, content string, fargs ...interface{}) {
    content = fmt.Sprintf(content, fargs...)
    g.debugf("(ClientError) (Subaction) (%s) %s", c.Identity(), content)
    if c.Identity().Type == simple.IdentityTypeBot {
        g.debugf("Bot (%s) bad subaction: '%s' table:%s", c.Identity(), content, g.table.JsonPretty())
    }
    c.Send(message.NewNotifySubactionError(header, content))
}

func (g *Game) notify(m message.Server) {
    for _, p := range g.players {
        p.Client.Send(m)
    }
    for _, o := range g.observers {
        o.Send(m)
    }
}

func min(x, y int) int {
    if x < y {
        return x
    }
    return y
}

func containsLocation(l simple.Location, ls []simple.Location) bool {
    for _, l2 := range ls {
        if l == l2 {
            return true
        }
    }
    return false
}

func (g *Game) tracef(msg string, fargs ...interface{}) {
    log.Trace(fmt.Sprintf("(G%d) %s", g.Id, msg), fargs...)
}

func (g *Game) debugf(msg string, fargs ...interface{}) {
    log.Debug(fmt.Sprintf("(G%d) %s", g.Id, msg), fargs...)
}

func (g *Game) infof(msg string, fargs ...interface{}) {
    log.Info(fmt.Sprintf("(G%d) %s", g.Id, msg), fargs...)
}

func (g *Game) warnf(msg string, fargs ...interface{}) {
    log.Warn(fmt.Sprintf("(G%d) %s", g.Id, msg), fargs...)
}

func (g *Game) errorf(msg string, fargs ...interface{}) {
    log.Error(fmt.Sprintf("(G%d) %s", g.Id, msg), fargs...)
}

func (g *Game) fatalf(msg string, fargs ...interface{}) {
    log.Fatal(fmt.Sprintf("(G%d) %s", g.Id, msg), fargs...)
}
