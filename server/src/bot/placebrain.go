package bot

import (
    "fmt"
    "local/hansa/log"
    "local/hansa/message"
    "local/hansa/simple"
)

type PlaceBrain struct {
    // Other configurable fields will go up here.
    identity simple.Identity
    gameId int

    player int
    table simple.Table
    handledBump bool
    iBumped bool
}

func (b *PlaceBrain) handleStartGame(d message.NotifyStartGameData) {
    b.table = d.Table

    for i, pb := range d.Table.PlayerBoards {
        if pb.Identity == b.identity {
            b.player = i
            break
        }
    }
}

// This convoluted if bool shit is because I conflated actions with what needs
// to happen next in a single message.
func (b *PlaceBrain) handleNotifySubaction(d message.NotifySubactionData) []message.Client {
    r := []message.Client{}
    if d.TurnState.Type != simple.Bumping {
        if d.TurnState.Player != b.player {
            b.table.ApplySubaction(d.Subaction, b.identity)
        }
        return r
    }

    if b.iBumped {
        b.iBumped = false
        return r
    }

    if d.TurnState.BumpingPlayer != b.player {
        b.table.ApplySubaction(d.Subaction, b.identity)
        return r
    }

    if b.handledBump {
        return r
    }

    b.handledBump = true
    return b.handleBump(d)
}

func (b *PlaceBrain) handleNotifyEndBump(d message.NotifyEndBumpData) []message.Client {
    b.handledBump = false
    if d.TurnState.Player == b.player {
        return b.handleNotifyNextTurn(message.NotifyNextTurnData{
            TurnState: d.TurnState,
            Elapsed: d.Elapsed,
        })
    }
    return []message.Client{}
}

func (b *PlaceBrain) handleNotifyNextTurn(d message.NotifyNextTurnData) []message.Client {
    r := []message.Client{}
    if d.TurnState.Player != b.player {
        return r
    }
    b.debugf("My turn")

    actions := d.TurnState.ActionsLeft
    newR, actions := b.placeOnRandomRoutes(actions)
    r = append(r, newR...)
    newR, actions = b.bags(actions)
    r = append(r, newR...)
    newR, actions = b.placeOnRandomRoutes(actions)
    r = append(r, newR...)
    newR, actions = b.move(actions)
    r = append(r, newR...)

    // TODO: more here (clear, bump).

    r = append(r, message.Client{
        CType: message.EndTurn,
        Data: message.EndTurnData{},
    })
    b.debugf("Decided: %v", r)
    return r
}

func (b *PlaceBrain) handleNotifySubactionError(d message.NotifySubactionErrorData) {
    b.debugf("I submitted a bad Subaction: %v", d)
}

// Using at max 'actions', place cubes then discs from supply on open spots
// until there are no spots left, or you have no pieces left.  Returns the
// number of actions used, and the Client messages to make those actions
// happen.  Mutates the table.
func (b *PlaceBrain) placeOnRandomRoutes(actions int) ([]message.Client, int) {
    r := []message.Client{}
    for _, p := range []simple.Piece{b.myCube(), b.myDisc()} {
        for supplyL, routeL := b.mySupply(p), b.emptyRouteSpot();
            supplyL != simple.NoneLocation && routeL != simple.NoneLocation && actions > 0;
            supplyL, routeL = b.mySupply(p), b.emptyRouteSpot() {

            subaction := simple.Subaction{
                Source: supplyL,
                Dest: routeL,
                Piece: p,
            }
            b.table.ApplySubaction(subaction, b.identity)
            r = append(r, message.Client{
                CType: message.DoSubaction,
                Data: subaction,
            })
            actions--
        }
    }
    return r, actions
}

// If actions is 0 or my stock is empty, do nothing.  Otherwise take a bags
// action to get as many pieces as I can, preferring discs.
func (b *PlaceBrain) bags(actions int) ([]message.Client, int) {
    r := []message.Client{}
    if actions == 0 {
        return r, 0
    }
    actions--

    bagsLeft := b.table.PlayerBoards[b.player].GetBags()

    for _, p := range []simple.Piece{b.myDisc(), b.myCube()} {
        for stockL, supplyL := b.myStock(p), b.mySupplyTarget();
            stockL != simple.NoneLocation && supplyL != simple.NoneLocation && bagsLeft > 0
            stockL, supplyL = b.myStock(p), b.mySupplyTarget() {

            subaction := simple.Subaction{
                Source: stockL,
                Dest: supplyL,
                Piece: p,
            }
            b.table.ApplySubaction(subaction, b.identity)
            r = append(r, message.Client{
                CType: message.DoSubaction,
                Data: subaction,
            })
            bagsLeft--
        }
    }
    return r, actions
}

// If actions is 0, do nothing.  Otherwise shuffle pieces around.
func (b *PlaceBrain) move(actions int) ([]message.Client, int) {
    return []message.Client{}, 0
}

func (b *PlaceBrain) handleBump(d message.NotifySubactionData) []message.Client {
    r := []message.Client{}
    do := func(a simple.Subaction) {
        b.table.ApplySubaction(a, b.identity)
        r = append(r, message.Client{
            CType: message.DoSubaction,
            Data: a,
        })
    }

    // TEMP
    b.debugf("Bot valid bump landing spots: %v", b.table.ValidBumps(d.TurnState.BumpingLocation))

    do(simple.Subaction{
        Source: d.TurnState.BumpingLocation,
        Dest: b.table.ValidBumps(d.TurnState.BumpingLocation)[0],
        Piece: b.table.GetPiece(d.TurnState.BumpingLocation),
    })

    for i:=0;i<d.TurnState.BumpingReplaces;i++ {
        if l := b.myStock(b.myDisc()); l != simple.NoneLocation {
            do(simple.Subaction{
                Source: l,
                Dest: b.table.ValidBumps(d.TurnState.BumpingLocation)[0],
                Piece: b.myDisc(),
            })
        } else if l := b.myStock(b.myCube()); l != simple.NoneLocation {
            do(simple.Subaction{
                Source: l,
                Dest: b.table.ValidBumps(d.TurnState.BumpingLocation)[0],
                Piece: b.myCube(),
            })
        } else if l := b.mySupply(b.myDisc()); l != simple.NoneLocation {
            do(simple.Subaction{
                Source: l,
                Dest: b.table.ValidBumps(d.TurnState.BumpingLocation)[0],
                Piece: b.myDisc(),
            })
        } else if l := b.mySupply(b.myCube()); l != simple.NoneLocation {
            do(simple.Subaction{
                Source: l,
                Dest: b.table.ValidBumps(d.TurnState.BumpingLocation)[0],
                Piece: b.myCube(),
            })
        }
    }

    r = append(r, message.Client{
        CType: message.EndBump,
        Data: message.EndBumpData{},
    })

    return r
}

func (b *PlaceBrain) emptyRouteSpot() simple.Location {
    for id, r := range b.table.Board.Routes {
        for index, s := range r.Spots {
            if s == (simple.Piece{}) {
                return simple.Location{
                    Type:simple.RouteLocationType,
                    Id: id,
                    Index: index,
                }
            }
        }
    }
    return simple.NoneLocation
}


func (b *PlaceBrain) myDisc() simple.Piece {
    return simple.Piece{
        b.table.PlayerBoards[b.player].Color,
        simple.DiscShape,
    }
}

func (b *PlaceBrain) myCube() simple.Piece {
    return simple.Piece{
        b.table.PlayerBoards[b.player].Color,
        simple.CubeShape,
    }
}

func (b *PlaceBrain) myStock(p simple.Piece) simple.Location {
    for i, p2 := range b.table.PlayerBoards[b.player].Stock {
        if p2 == p {
            return simple.Location{
                Type: simple.PlayerLocationType,
                Id: b.player,
                Index: 5,
                Subindex: i,
            }
        }
    }
    return simple.NoneLocation
}

func (b *PlaceBrain) mySupply(p simple.Piece) simple.Location {
    for i, p2 := range b.table.PlayerBoards[b.player].Supply {
        if p2 == p {
            return simple.Location{
                Type: simple.PlayerLocationType,
                Id: b.player,
                Index: 6,
                Subindex: i,
            }
        }
    }
    return simple.NoneLocation
}

func (b *PlaceBrain) mySupplyTarget() simple.Location {
    for i, p2 := range b.table.PlayerBoards[b.player].Supply {
        if p2 == (simple.Piece{}) {
            return simple.Location{
                Type: simple.PlayerLocationType,
                Id: b.player,
                Index: 6,
                Subindex: i,
            }
        }
    }
    return simple.NoneLocation
}

func (b *PlaceBrain) debugf(msg string, fargs ...interface{}) {
    log.Debug(fmt.Sprintf("(G%d) (Bot%s) (P%d) %s", b.gameId, b.identity, b.player, msg), fargs...)
}
