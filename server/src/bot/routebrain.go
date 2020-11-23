package bot

import (
    "encoding/json"
    "fmt"
    "sort"
    "local/hansa/log"
    "local/hansa/message"
    "local/hansa/simple"
)

type RouteBrain struct {
    // Required on instantiation
    identity simple.Identity
    gameId int
    weights WeightSet

    // Initialized when we get a startgame message
    player int
    color simple.PlayerColor
    table simple.Table
    scores []int

    // When we are bumped, we respond and set this to true.  This is necessary
    // because of NotifySubaction's conflation of current state and what it
    // needs next.
    handledBump bool

    // When we are bumping mid plan, we remember the remainder of our plan
    // here.  When we gain back control, we continue blindly with this plan,
    // and then if we have actions left make new plans.
    postBumpPlan Plan
}

func (b *RouteBrain) handleStartGame(d message.NotifyStartGameData) {
    b.table = d.Table
    for i, pb := range d.Table.PlayerBoards {
        if pb.Identity == b.identity {
            b.player = i
            b.color = pb.Color
        }
        b.scores = append(b.scores, 0)
    }
    b.debugf("The game is starting!  There are %d players and I am %s",
        len(d.Table.PlayerBoards), simple.PlayerColorNames[b.color])
}

// This convoluted if bool shit is because I conflated actions with what needs
// to happen next in a single message.
func (b *RouteBrain) handleNotifySubaction(d message.NotifySubactionData) []message.Client {
    r := []message.Client{}
    for i, s := range d.Scores {
        b.scores[i] += s
    }
    b.applySubaction(d.Subaction)

    // There is only one time when a handleNotifySubaction requires a response:
    // I was just bumped.  So as long as we're not in a bumping state, or even
    // if we are if it's not me who was bumped, or even if it was if I didn't
    // already respond to this bump and am seeing my own actions, we're done.
    if d.TurnState.Type != simple.Bumping || d.TurnState.BumpingPlayer != b.player || b.handledBump {
        return r
    }

    // When I get bumped, I set b.handledBump so that when I see my own actions
    // through handleNotifySubaction (right here, later) I don't re-reply.
    b.handledBump = true
    return b.handleBump(d)
}

func (b *RouteBrain) handleNotifyNextTurn(d message.NotifyNextTurnData) []message.Client {
    r := []message.Client{}
    if d.TurnState.Player != b.player {
        return r
    }
    b.debugf("My turn is beginning")
    return b.chooseAndExecutePlans(d.TurnState.ActionsLeft)
}

func (b *RouteBrain) handleNotifyEndBump(d message.NotifyEndBumpData) []message.Client {
    b.handledBump = false
    if d.TurnState.Player == b.player {
        b.debugf("I have control back after my bump.")

        // If I have no actions left, I just need to end turn.
        if d.TurnState.ActionsLeft == 0 {
            b.debugf("I have 0 actions left, so I'm done")
            return []message.Client{message.Client{
                CType: message.EndTurn,
                Data: message.EndTurnData{},
            }}
        }

        // If I have no post bump plan, let's calculate new plan(s) from
        // scratch and execute.
        plan := b.postBumpPlan
        b.postBumpPlan = Plan{}
        if plan.Goal == NoneGoal {
            return b.chooseAndExecutePlans(d.TurnState.ActionsLeft)
        }

        b.errorf("Post bump plan not handled yet.")
        return []message.Client{message.Client{
            CType: message.EndTurn,
            Data: message.EndTurnData{},
        }}
    }
    return []message.Client{}
}

func (b *RouteBrain) chooseAndExecutePlans(actionsLeft int) []message.Client {
    plans := []Plan{}
    b.debugf("I have %d actions", actionsLeft)

    bump := false
    for ;actionsLeft > 0 && !bump; {
        plan := b.choosePlan(actionsLeft)
        plans = append(plans, plan)
        if plan.Actions == 0 {
            b.errorf("Generated no Actions plan, just ending turn: %v", plan)
            actionsLeft = 0
        } else {
            actionsLeft -= plan.Actions
        }
        bump = len(plan.Bumps) > 0
    }

    // At this point, we know entirely what we will do until the end of our
    // turn, or until bump someone and hand off control.  Let's undo it from
    // our board state so that we can do it naturally like the other players on
    // receipt of NotifySubaction.
    toUndo := []simple.Subaction{}
    for _, p := range plans {
        newUndos := p.Subactions
        if len(p.Bumps) > 0 {
            newUndos = newUndos[0:p.Bumps[0]+1]
        }
        toUndo = append(toUndo, newUndos...)
    }
    b.table.UndoSubactions(toUndo)

    return b.executePlansUntilBump(plans)
}

// Note that this will mutate b.table with the chosen subactions, up until
// paying for a bump (if one occurs).  Future subactions will exist in the plan
// but not yet be applied to b.table.
func (b *RouteBrain) choosePlan(actions int) Plan {
    b.debugf("choosePlan for %d actions", actions)
    c := b.buildContext(actions)

    b.debugf("Generating plans with context %+v", c)
    plans := []Plan{}
    for _, r := range b.table.Board.Routes {
        for _, g := range allGoals {
            plans = append(plans, b.generatePlans(r.Id, g, c)...)
        }
    }
    sort.Slice(plans, func(i, j int) bool {
        return plans[i].FitnessValue > plans[j].FitnessValue
    })
    b.debugf("Considered %d potential plans", len(plans))
    b.debugf("Chose a %s plan to %s on route %d with fitness %d",
        lengthNames[plans[0].Length], goalNames[plans[0].Goal], plans[0].RouteId, plans[0].FitnessValue)
    b.debugf("Fitness calculation: %s", plans[0].FitnessDescription)
    b.debugf("Full plan: %+v", plans[0])

    // This is where we mutate b.table chosen plan (we stop after applying bump
    // payment if it exists).
    for i, s := range plans[0].Subactions {
        b.applySubaction(s)
        b.debugf("Local Apply: %+v", s)
        if len(plans[0].Bumps) > 0 && plans[0].Bumps[0] == i {
            break
        }
    }

    return plans[0]
}

func (b *RouteBrain) buildContext(actions int) Context {
    c := Context{ActionsLeft: actions}

    for _, r := range b.table.Board.Routes {
        for _, p := range r.Spots {
            if p.PlayerColor == b.color {
                if p.Shape == simple.DiscShape {
                    c.BoardDisc = true
                }
                c.BoardPieces++
                c.LivePieces++
            }
        }
    }
    for _, p := range b.table.PlayerBoards[b.player].Stock {
        if p != (simple.Piece{}) {
            if p.Shape == simple.DiscShape {
                c.StockDisc = true
            }
            c.StockPieces++
            c.LivePieces++
        }
    }
    for _, p := range b.table.PlayerBoards[b.player].Supply {
        if p != (simple.Piece{}) {
            if p.Shape == simple.DiscShape {
                c.SupplyDisc = true
            }
            c.SupplyPieces++
            c.LivePieces++
        }
    }

    maxScore := 0
    for _, s := range b.scores {
        if s > maxScore {
            maxScore = s
        }
    }
    if maxScore < 5 {
        c.GameTime = EarlyGame
    } else if maxScore < 15 {
        c.GameTime = MidGame
    } else {
        c.GameTime = LateGame
    }

    return c
}

// This can not mutate b.table nor c permanently.  It may mutate b.table while
// thinking about a plan, but should always undo everything.
func (b *RouteBrain) generatePlans(r int, g Goal, c Context) []Plan {
    ret := []Plan{}
    before := b.serializeTable()
    switch g {
        case AwardGoal:
            ret = b.generateAwardPlans(r, c)
        case OfficeGoal:
            ret = b.generateOfficePlans(r, c)
        case PointsGoal:
            ret = b.generatePointsPlans(r, c, simple.NoneShape)
        case BlockGoal:
            ret = b.generateBlockPlans(r, c)
    }
    after := b.serializeTable()
    if before != after {
        panic(fmt.Sprintf("RouteBrain mutated table! r=%d g=%d c=%v before=%s, after=%s",
            r, g, c, before, after))
    }
    return ret
}

func (b *RouteBrain) generateAwardPlans(r int, c Context) []Plan {
    a := b.getRouteAward(r)
    weight := b.rawAwardWeight(a, c)
    if weight < 0.0001 {
        return []Plan{}
    }

    shape := simple.NoneShape
    if a == simple.CoellenAward {
        shape = simple.DiscShape
    }

    // Note that the subactions in each of these plans are not currently
    // applied to b.table.
    ps := b.generatePointsPlans(r, c, shape)
    for i, _ := range ps {
        ps[i].Goal = AwardGoal

        // Move over the pieces of PointsPlanFitness that we care about, and
        // recalculate our own fitness with the necessary fields.
        fitness := AwardPlanFitness{
            Length: ps[i].Fitness.(PointsPlanFitness).Length,
            BumpInfos: ps[i].Fitness.(PointsPlanFitness).BumpInfos,
            MyPoints: ps[i].Fitness.(PointsPlanFitness).MyPoints,
            OthersPoints: ps[i].Fitness.(PointsPlanFitness).OthersPoints,
        }
        fitness.Award = a
        if a == simple.CoellenAward {
            fitness.AwardsLeft = b.getCoellenTarget().Index
        } else {
            fitness.AwardsLeft = b.table.PlayerBoards[b.player].AwardTrackRemaining(a)
        }
        ps[i].Fitness = fitness
        ps[i].FitnessValue = fitness.Value(b.weights[c.GameTime])
        ps[i].FitnessDescription = fitness.Calculation(b.weights[c.GameTime])

        // If we cleared, we also need to take our reward.
        if ps[i].Length == ShortPlan || ps[i].Length == FullPlan {

            // The coellen reward is very similar to taking a disc office.  We
            // look for a clear subaction with a disc (guaranteed because we
            // passed disc=true to generatePointsPlans) and replace the
            // destination with the coellen table.
            if a == simple.CoellenAward {
                for i2:=len(ps[i].Subactions)-1;i2>=0;i2-- {
                    candidate := ps[i].Subactions[i2]
                    if candidate.Dest.Type == simple.PlayerLocationType &&
                        candidate.Piece.Shape == simple.DiscShape {
                        ps[i].Subactions[i2].Dest = b.getCoellenTarget()
                        break
                    }
                }

            // All other rewards require us to move a new piece from a
            // playerboard area to the supply.  In order to know which
            // subindexes in our supply are open, we need to apply all previous
            // subactions.
            } else {
                b.applySubactions(ps[i].Subactions)
                index, subindex := b.table.PlayerBoards[b.player].AwardClearLocation(a)
                piece := b.myCube()
                if a == simple.DiscsAward {
                    piece = b.myDisc()
                }
                s := simple.Subaction{
                    Source: simple.Location{
                        Type: simple.PlayerLocationType,
                        Id: b.player,
                        Index: index,
                        Subindex: subindex,
                    },
                    Dest: b.myOpenSupply(),
                    Piece: piece,
                }
                b.table.UndoSubactions(ps[i].Subactions)
                ps[i].Subactions = append(ps[i].Subactions, s)
            }
        }
    }
    return ps
}


// Returns None location if we can't take an office here without getting
// another upgrade (no priviledge, full).
func (b *RouteBrain) getBuildOfficeInfo(city simple.City, c Context) (simple.Location, simple.Shape) {
    myPriviledge := b.table.PlayerBoards[b.player].GetPriviledge()

    for i, o := range city.Offices {
        if o.Piece == (simple.Piece{}) {
            if myPriviledge < o.Priviledge {
                return simple.NoneLocation, simple.NoneShape
            }
            return simple.Location{
                Type: simple.CityLocationType,
                Id: city.Id,
                Index: i,
            }, o.Shape
        }
    }
    return simple.NoneLocation, simple.NoneShape
}



/*
}
*/

func (b *RouteBrain) generateOfficePlans(r int, c Context) []Plan {
    returnPlans := []Plan{}

    // Generate up to 2 plans, one for claiming an office in each city (left and right).
    for _, city := range []simple.City{
        b.table.Board.Cities[b.table.Board.Routes[r].LeftCityId],
        b.table.Board.Cities[b.table.Board.Routes[r].RightCityId],
    } {
        location, shape := b.getBuildOfficeInfo(city, c)
        if location == simple.NoneLocation {
            continue
        }
        if shape == simple.DiscShape && !c.SupplyDisc && !c.StockDisc {
            continue
        }

        // Note that the subactions in each of these plans are not currently
        // applied to b.table.
        ps := b.generatePointsPlans(r, c, shape)
        for i, _ := range ps {
            ps[i].Goal = OfficeGoal

            // Steal the parts of PointsPlanFitness that we want to use.
            fitness := OfficePlanFitness{
                Length: ps[i].Fitness.(PointsPlanFitness).Length,
                BumpInfos: ps[i].Fitness.(PointsPlanFitness).BumpInfos,
                MyPoints: ps[i].Fitness.(PointsPlanFitness).MyPoints,
                OthersPoints: ps[i].Fitness.(PointsPlanFitness).OthersPoints,
            }
            fitness.AwardOffice = city.Award != simple.NoneAward
            fitness.DiscOffice = shape == simple.DiscShape

            // Calculate fitness pieces that are specific to building an office.
            presence := map[simple.PlayerColor]int{}
            presenceTiebreak := map[simple.PlayerColor]int{}
            for i, o := range city.Offices {
                if o.Piece != (simple.Piece{}) {
                    presence[o.Piece.PlayerColor] += 1
                    presenceTiebreak[o.Piece.PlayerColor] = i
                }
            }
            fitness.FirstOffice = len(presence) == 0

            // Calculating whether this would give me control is a little
            // convoluted.
            myControl := presence[b.color]
            winningControl := 0
            winningTie := false
            iWinTie := false
            for color, v := range presence {
                if v == winningControl {
                    winningTie = true
                    if color == b.color {
                        iWinTie = true
                    } else {
                        iWinTie = false
                    }
                }
                if v > winningControl {
                    winningControl = v
                    winningTie = false
                    if color == b.color {
                        iWinTie = true
                    } else {
                        iWinTie = false
                    }
                }
            }
            fitness.NonControlOffice = winningControl != 0 &&
                myControl == winningControl - 1 &&
                !(myControl == winningControl && winningTie && !iWinTie)
            fitness.NetworkDelta = b.table.Board.GetNetworkScoreIfCity(b.color, city.Id) -
                b.table.Board.GetNetworkScore(b.color)
            fitness.NetworkDelta = min(fitness.NetworkDelta, 6)
            ps[i].Fitness = fitness
            ps[i].FitnessValue = fitness.Value(b.weights[c.GameTime])
            ps[i].FitnessDescription = fitness.Calculation(b.weights[c.GameTime])

            // If we cleared, we need to replace one of the clearing Subactions
            // with a move into our office.  If we took a disc office, we need to
            // move a disc, and if we took a cube office, we need to move a cube.
            if ps[i].Length == ShortPlan || ps[i].Length == FullPlan {

                // We are looking for a clear subaction with the right shape
                handled := false
                for i2:=len(ps[i].Subactions)-1;i2>=0;i2-- {
                    candidate := ps[i].Subactions[i2]
                    if candidate.Source.Type == simple.RouteLocationType &&
                        candidate.Source.Id == r &&
                        candidate.Piece.Shape == shape {
                        ps[i].Subactions[i2].Dest = location
                        handled = true
                        break
                    }
                }

                // TODO: Note there is a rare edge case here where the cleared route is
                // all discs and we want to take a cube office.  In that case we
                // just bomb out for now.
                if !handled {
                    return []Plan{}
                }
            }
        }
        returnPlans = append(returnPlans, ps...)
    }

    // TODO: Revisit when computer are moving pieces again.
    if len(returnPlans) < 2 {
        return returnPlans
    } else if returnPlans[0].FitnessValue > returnPlans[1].FitnessValue {
        return []Plan{returnPlans[0]}
    }
    return []Plan{returnPlans[1]}
}

// Unlike the other generate*Plan methods, this is guaranteed to return a plan
// when shape is NoneShape. (no matter how bad, even if it generates 0 points).
// This way in degenerate board states where there is somehow no awards to
// claim, no offices I'm elligible for, no points to gain, and no other players
// to block (???) we still have a plan.
func (b *RouteBrain) generatePointsPlans(r int, c Context, shape simple.Shape) []Plan {
    fitness := PointsPlanFitness{}

    // Each route spot will need to be filled via place or bump.
    stockAndSupply := min(10, b.myStockAndSupplyCount())
    discToBump := []simple.Location{}
    cubeToBump := []simple.Location{}
    openSpot := []simple.Location{}
    for i, p := range b.table.Board.Routes[r].Spots {
        if p.PlayerColor == b.color {
            continue
        }
        l := simple.Location{
            Type: simple.RouteLocationType,
            Id: r,
            Index: i,
            Subindex: 0,
        }
        if p.PlayerColor == simple.NonePlayerColor {
            openSpot = append(openSpot, l)
        } else {
            if p.Shape == simple.DiscShape {
                discToBump = append(discToBump, l)
                stockAndSupply = max(0, stockAndSupply - 2)
            } else {
                cubeToBump = append(cubeToBump, l)
                stockAndSupply = max(0, stockAndSupply - 1)
            }
            fitness.BumpInfos = append(fitness.BumpInfos, BumpFitnessInfo{
                Disc: p.Shape == simple.DiscShape,
                Bags: b.table.PlayerBoards[b.player].GetBags(),
                StockAndSupply: stockAndSupply,
            })
        }
    }

    // Before we start mutating anything with this plan, let's calculate how
    // many points clearing this route will give and opponents.
    lc := b.table.Board.Cities[b.table.Board.Routes[r].LeftCityId].GetControl()
    rc := b.table.Board.Cities[b.table.Board.Routes[r].RightCityId].GetControl()
    if lc == b.color {
        fitness.MyPoints++
    } else if lc != simple.NonePlayerColor {
        fitness.OthersPoints++
    }
    if rc == b.color {
        fitness.MyPoints++
    } else if rc != simple.NonePlayerColor {
        fitness.OthersPoints++
    }

    // Fill the route with our pieces.  First fill open spots, then bump cubes,
    // then bump discs.  If we need a shape on the route for clearing reasons,
    // place that piece first.
    actionsLeft := c.ActionsLeft
    complete := true
    subactions := []simple.Subaction{}
    bumps := []int{}
    for _, o := range openSpot {
        if actionsLeft == 0 {
            complete = false
            break
        }
        ss, used, finished := b.placePieceBagsIfNecessary(o, shape, actionsLeft == 1)
        shape = simple.NoneShape
        actionsLeft -= used
        subactions = append(subactions, ss...)
        if !finished {
            complete = false
            break
        }
    }
    for _, o := range cubeToBump {
        if actionsLeft == 0 || !complete {
            complete = false
            break
        }
        ss, used, finished := b.bumpPieceBagsIfNecessary(o, shape, simple.CubeShape, actionsLeft == 1)
        shape = simple.NoneShape
        actionsLeft -= used
        subactions = append(subactions, ss...)
        if !finished {
            complete = false
            break
        }
        // If we were able to finish the bumpPiece, the final subaction is the
        // final payment for the bump.  This is where we will have to hand off
        // control to other players.
        bumps = append(bumps, len(subactions)-1)
    }
    for _, o := range discToBump {
        if actionsLeft == 0 || !complete {
            complete = false
            break
        }
        ss, used, finished := b.bumpPieceBagsIfNecessary(o, shape, simple.DiscShape, actionsLeft == 1)
        shape = simple.NoneShape
        actionsLeft -= used
        subactions = append(subactions, ss...)
        if !finished {
            complete = false
            break
        }
        // If we were able to finish the bumpPiece, the final subaction is the
        // final payment for the bump.  This is where we will have to hand off
        // control to other players.
        bumps = append(bumps, len(subactions)-1)
    }
    routeFilled := false
    routeCleared := false
    if complete {
        routeFilled = true
    }
    if actionsLeft != 0 {
        // If this is false, we still have actions left, but we don't have
        // enough pieces to finish bumping/placing on the route.  Our Plan will
        // be of Length Uncompletable below.
        if complete {
            actionsLeft--
            routeCleared = true
            subactions = append(subactions, b.clearRouteForPoints(r)...)
        }
    }

    length := LongPlan
    if actionsLeft > 0 {
        if complete {
            length = ShortPlan
        } else {
            length = UncompletablePlan
        }
    } else if routeCleared {
        length = FullPlan
    } else if routeFilled {
        length = AlmostPlan
    }
    fitness.Length = length

    b.table.UndoSubactions(subactions)
    return []Plan{Plan{
        RouteId: r,
        Goal: PointsGoal,
        Length: length,
        Actions: c.ActionsLeft - actionsLeft,
        Moves: false,
        Fitness: fitness,
        FitnessValue: fitness.Value(b.weights[c.GameTime]),
        FitnessDescription: fitness.Calculation(b.weights[c.GameTime]),
        Subactions: subactions,
        Bumps: bumps,
    }}
}

// Mutates.  Returns the subactions that were applied to b.table, how many
// actions it used, and whether it was able to finish what it wanted to do.  If
// only1action is true and we had to bags before placing, we will just bags and
// return (_, _, false).  If there are not enough pieces in Stock+Supply to
// bump and pay for the bump we return ([]Subaction, 0, false)
func (b *RouteBrain) bumpPieceBagsIfNecessary(dest simple.Location, shape simple.Shape, bumpedShape simple.Shape, only1action bool) (
    []simple.Subaction, int, bool) {
    r := []simple.Subaction{}

    actions := 1
    needed := 2
    if bumpedShape == simple.DiscShape {
        needed = 3
    }
    supply := 0
    shapeFulfilled := shape == simple.NoneShape
    for _, p := range b.table.PlayerBoards[b.player].Supply {
        if p != (simple.Piece{}) {
            supply++
            if shape != simple.NoneShape && !shapeFulfilled && p.Shape == shape {
                shapeFulfilled = true
            }
        }
    }
    stock := 0
    if supply < needed || !shapeFulfilled {
        for _, p := range b.table.PlayerBoards[b.player].Stock {
            if p != (simple.Piece{}) {
                stock++
                if shape != simple.NoneShape && !shapeFulfilled && p.Shape == shape {
                    shapeFulfilled = true
                }
            }
        }
        if supply+stock < needed || !shapeFulfilled {
            return r, 0, false
        }

        // If we need a piece and don't have one, or if we don't need anything
        // specific but have 0 pieces, we don't do anything.
        ss := b.bags(shape)
        if len(ss) == 0 {
            return r, 0, false
        }
        r = append(r, ss...)

        if only1action {
            return r, 1, false
        }
        actions++
    }

    // Make the bump
    var p simple.Piece
    var source simple.Location
    if shape == simple.DiscShape {
        p = b.myDisc()
        source = b.mySupply(p)
    } else if shape == simple.CubeShape {
        p = b.myCube()
        source = b.mySupply(p)
    } else {
        p = b.myCube()
        source = b.mySupply(p)
        if source == simple.NoneLocation {
            p = b.myDisc()
            source = b.mySupply(p)
        }
    }
    s := simple.Subaction{
        Source: source,
        Dest: dest,
        Piece: p,
    }
    b.applySubaction(s)
    r = append(r, s)

    // Pay for it
    for i:=0;i<needed-1;i++ {
        piece := b.myCube()
        source := b.mySupply(piece)
        if source == simple.NoneLocation {
            piece = b.myDisc()
            source = b.mySupply(piece)
        }
        if source == simple.NoneLocation {
            json := b.table.JsonPretty()
            panic (fmt.Sprintf("Bot invariant fail: can't pay for bump: "+
                "needed:%d, supply:%d, stock:%d, r:%v only1action:%t Table: %s", needed, supply, stock, r, only1action, json))
        }
        s := simple.Subaction{
            Source: source,
            Dest: b.myOpenStock(),
            Piece: piece,
        }
        b.applySubaction(s)
        r = append(r, s)
    }

    return r, actions, true
}

func (b *RouteBrain) executePlansUntilBump(plans []Plan) []message.Client {
    r := []message.Client{}
    for _, p := range plans {
        for i, s := range p.Subactions {
            r = append(r, message.Client{
                CType: message.DoSubaction,
                Data: s,
            })

            // If the subaction I just did is a bump, I need to return this
            // list without ending my turn, after storing the rest of the plan
            // for post bump.  Note that if the plan includes a bump it's
            // always the last plan, because we can't think beyond our
            // opponents bump response and our current plan.
            if len(p.Bumps) > 0 && p.Bumps[0] == i {
                p.Subactions = p.Subactions[i+1:]
                p.Bumps = p.Bumps[1:]
                // TODO: Not handling, we recalc for now.
                //b.postBumpPlan = p
                return r
            }
        }
    }
    r = append(r, message.Client{
        CType: message.EndTurn,
        Data: message.EndTurnData{},
    })
    return r
}

// Assumes we are complete on the route, and have an action.
func (b *RouteBrain) clearRouteForPoints(r int) []simple.Subaction {
    ss := []simple.Subaction{}
    for i, spot := range b.table.Board.Routes[r].Spots {
        s := simple.Subaction{
            Source: simple.Location{
                Type: simple.RouteLocationType,
                Id: r,
                Index: i,
                Subindex: 0,
            },
            Dest: b.myOpenStock(),
            Piece: spot,
        }
        b.applySubaction(s)
        ss = append(ss, s)
    }
    return ss
}

// It's assumed that we are never called with c.ActionsLeft == 0
func (b *RouteBrain) generateBlockPlans(r int, c Context) []Plan {
    fitness := BlockPlanFitness{
        Discs: b.table.PlayerBoards[b.player].GetBookDiscs(),
        StockAndSupply: min(b.myStockAndSupplyCount(), 10),
    }

    // We can only block if we are not on the route, and if someone else is on
    // the route, and if there is 1+ open spot.  Note that the goal is to get
    // bumped; if we actually want the route, generatePointsPlans or some
    // derivative will score highly, and we will use that plan instead.
    someoneElse := false
    openSpots := []simple.Location{}
    for i, s := range b.table.Board.Routes[r].Spots {
        if s.PlayerColor == simple.NonePlayerColor {
            openSpots = append(openSpots, simple.Location{
                Type: simple.RouteLocationType,
                Id: r,
                Index: i,
                Subindex: 0,
            })
        } else if s.PlayerColor == b.color {
            return []Plan{}
        } else {
            if someoneElse {
                fitness.DoublePlayer = true
            }
            someoneElse = true
        }
    }
    if !someoneElse || len(openSpots) == 0 {
        return []Plan{}
    }
    fitness.DoublePiece = len(openSpots) > 1

    // Note we will always have the pieces in Supply+Stock because of our
    // opening check in this function.
    // TODO: We could block with a move as well, which would be useful
    actionsLeft := c.ActionsLeft
    complete := true
    subactions := []simple.Subaction{}
    for _, o := range openSpots {
        if actionsLeft == 0 {
            complete = false
            break
        }
        ss, used, finished := b.placePieceBagsIfNecessary(o, simple.NoneShape, actionsLeft == 1)
        actionsLeft -= used
        subactions = append(subactions, ss...)
        if !finished {
            complete = false
            break
        }
    }

    length := FullPlan
    if !complete {
        length = LongPlan
    } else if actionsLeft > 0 {
        length = ShortPlan
    }
    fitness.Length = length

    // TODO: This is a simplification.  For now we add in how much we would
    // like to get a reward or an office on this route if we could, and use
    // that for how much we like to block someone else.
    fitness.OpponentDesire = maxFloat(b.rawAwardWeight(b.getRouteAward(r), c), b.weights[c.GameTime].Office)

    b.table.UndoSubactions(subactions)
    return []Plan{Plan{
        RouteId: r,
        Goal: BlockGoal,
        Length: length,
        Actions: c.ActionsLeft - actionsLeft,
        Moves: false,
        Fitness: fitness,
        FitnessValue: fitness.Value(b.weights[c.GameTime]),
        FitnessDescription: fitness.Calculation(b.weights[c.GameTime]),
        Subactions: subactions,
        Bumps: []int{},
    }}
}

func (b *RouteBrain) getRouteAward(r int) simple.Award {
    award := b.table.Board.Cities[b.table.Board.Routes[r].LeftCityId].Award
    if award != simple.NoneAward {
        return award
    }
    return b.table.Board.Cities[b.table.Board.Routes[r].RightCityId].Award
}

// If the route has no award, this is 0.  If we can not gain the award (track
// fully leveled or no Disc for Coellen), this is 0.  If the route has an
// award, we pull the appropriate weight from our brain based on our
// PlayerBoard.
func (b *RouteBrain) rawAwardWeight(a simple.Award, c Context) float64 {
    if a == simple.NoneAward {
        return 0.0
    }
    if a == simple.CoellenAward {
        if !(c.StockDisc || c.SupplyDisc || c.BoardDisc) {
            return 0.0
        }
        l := b.getCoellenTarget()
        if l == simple.NoneLocation {
            return 0.0
        }
        return b.weights[c.GameTime].Awards[simple.CoellenAward][l.Index]
    }
    track := b.table.PlayerBoards[b.player].AwardTrackRemaining(a)
    if track == 0 {
        return 0.0
    }
    return b.weights[c.GameTime].Awards[a][track]
}

// Gets the highest coellen spot with priviledge that I have.  Returns
// NoneLocation if there isn't one.
func (b *RouteBrain) getCoellenTarget() simple.Location {
    myPriviledge := b.table.PlayerBoards[b.player].GetPriviledge()
    var r simple.Location
    for i, c := range b.table.Board.Cities {
        for index, s := range c.Coellen.Spots {
            if s.Priviledge <= myPriviledge && s.Piece == (simple.Piece{}) {
                r = simple.Location{
                    Type: simple.CityLocationType,
                    Id: i,
                    Index: index,
                    Subindex: 2,
                }
            }
        }
    }
    return r
}

// Mutates.  Returns the subactions that were applied to b.table, how many
// actions it used, and whether it was able to finish what it wanted to do.  If
// only1action is true and we had to bags before placing, we will just bags and
// return (_, _, false).  If there are 0 pieces in Stock+Supply we return
// ([]Subaction, 0, false)
func (b *RouteBrain) placePieceBagsIfNecessary(dest simple.Location, shape simple.Shape, only1action bool) ([]simple.Subaction, int, bool) {
    r := []simple.Subaction{}
    actions := 1

    // Get a valid piece from the supply
    var p simple.Piece
    var l simple.Location
    if shape == simple.DiscShape {
        p = b.myDisc()
        l = b.mySupply(p)
    } else if shape == simple.CubeShape {
        p = b.myCube()
        l = b.mySupply(p)
    } else {
        p = b.myCube()
        l = b.mySupply(p)
        if l == simple.NoneLocation {
            p = b.myDisc()
            l = b.mySupply(p)
        }
    }

    // If we couldn't find one, we have to bags.
    if l == simple.NoneLocation {

        // If we need a piece and don't have one, or if we don't need anything
        // specific but have 0 pieces, we don't do anything.
        ss := b.bags(shape)
        if len(ss) == 0 {
            return r, 0, false
        }

        // If we were only allowed to take 1 action, we don't have time to
        // place this piece.
        r = append(r, ss...)
        if only1action {
            return r, 1, false
        }

        actions = 2

        if shape == simple.DiscShape {
            p = b.myDisc()
            l = b.mySupply(p)
        } else if shape == simple.CubeShape {
            p = b.myCube()
            l = b.mySupply(p)
        } else {
            p = b.myCube()
            l = b.mySupply(p)
            if l == simple.NoneLocation {
                p = b.myDisc()
                l = b.mySupply(p)
            }
        }

        // We are sure l is set, because bags returned subactions aboe.
    }

    s := simple.Subaction{
        Source: l,
        Dest: dest,
        Piece: p,
    }
    b.applySubaction(s)
    r = append(r, s)
    return r, actions, true
}

// Mutates.  If there are 0 pieces in Stock it will return an
// empty []Subaction.
func (b *RouteBrain) bags(shape simple.Shape) []simple.Subaction {
    r := []simple.Subaction{}
    bags := b.table.PlayerBoards[b.player].GetBags()

    // If there is a required shape, get that one first.
    if shape != simple.NoneShape {
        var p simple.Piece
        if shape == simple.DiscShape {
            p = b.myDisc()
        } else {
            p = b.myCube()
        }
        l := b.myStock(p)
        if l == simple.NoneLocation {
            return r
        }
        s := simple.Subaction{
            Source: l,
            Dest: b.myOpenSupply(),
            Piece: p,
        }
        b.applySubaction(s)
        r = append(r, s)
        bags--
    }

    // For other shapes (or if none were required) prefer discs over bags.
    // Leaving discs in stock for bump response requires look ahead and is out
    // of scope.
    for i:=0;i<bags;i++ {
        p := b.myDisc()
        l := b.myStock(p)
        if l == simple.NoneLocation {
            p = b.myCube()
            l = b.myStock(p)
        }
        if l == simple.NoneLocation {
            return r
        }
        s := simple.Subaction{
            Source: l,
            Dest: b.myOpenSupply(),
            Piece: p,
        }
        b.applySubaction(s)
        r = append(r, s)
    }
    return r
}

func (b *RouteBrain) myOpenStock() simple.Location {
    for i, s := range b.table.PlayerBoards[b.player].Stock {
        if s == (simple.Piece{}) {
            return simple.Location{
                Type: simple.PlayerLocationType,
                Id: b.player,
                Index: 5,
                Subindex: i,
            }
        }
    }
    // Should never be hit... we have more stock positions than pieces in the
    // game.
    return simple.NoneLocation
}

func (b *RouteBrain) myOpenSupply() simple.Location {
    for i, s := range b.table.PlayerBoards[b.player].Supply {
        if s == (simple.Piece{}) {
            return simple.Location{
                Type: simple.PlayerLocationType,
                Id: b.player,
                Index: 6,
                Subindex: i,
            }
        }
    }
    // Should never be hit... we have more supply positions than pieces in the
    // game.
    return simple.NoneLocation
}

func (b *RouteBrain) handleNotifySubactionError(d message.NotifySubactionErrorData) {
    b.errorf("I submitted a bad Subaction: '%v' table: %s", d, b.table.JsonPretty())
}

// Note this should not permanently mutate b.table; we only permanently mutate
// b.table when we receive NotifySubaction messages, as this is the simplest
// way to keep our representation valid.
func (b *RouteBrain) handleBump(d message.NotifySubactionData) []message.Client {

    r := []message.Client{}
    toUndo := []simple.Subaction{}
    do := func(a simple.Subaction) {
        toUndo = append(toUndo, a)
        b.applySubaction(a)
        r = append(r, message.Client{
            CType: message.DoSubaction,
            Data: a,
        })
    }

    /*
    // TODO: This.
    routes := map[int]simple.Route{}
    for _, l := range b.table.ValidBumps(d.TurnState.BumpingLocation) {
        routes[l.Id] = b.table.Board.Routes[l.Id]
    }

    routeScores := map[int]int{}
    for _, route := range routes {
        a := b.getRouteAward(r)
    }
    */

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

    b.table.UndoSubactions(toUndo)
    r = append(r, message.Client{
        CType: message.EndBump,
        Data: message.EndBumpData{},
    })
    return r
}

func (b *RouteBrain) myStockAndSupplyCount() int {
    return b.myStockCount() + b.mySupplyCount()
}

func (b *RouteBrain) myStockCount() int {
    r := 0
    for _, p := range b.table.PlayerBoards[b.player].Stock {
        if p.PlayerColor == b.color {
            r++
        }
    }
    return r
}

func (b *RouteBrain) mySupplyCount() int {
    r := 0
    for _, p := range b.table.PlayerBoards[b.player].Supply {
        if p.PlayerColor == b.color {
            r++
        }
    }
    return r
}

func (b *RouteBrain) myDisc() simple.Piece {
    return simple.Piece{
        b.table.PlayerBoards[b.player].Color,
        simple.DiscShape,
    }
}

func (b *RouteBrain) myCube() simple.Piece {
    return simple.Piece{
        b.table.PlayerBoards[b.player].Color,
        simple.CubeShape,
    }
}

func (b *RouteBrain) myStock(p simple.Piece) simple.Location {
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

func (b *RouteBrain) mySupply(p simple.Piece) simple.Location {
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

func (b *RouteBrain) mySupplyTarget() simple.Location {
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

func (b *RouteBrain) applySubactions(ss []simple.Subaction) {
    for _, s := range ss {
        b.applySubaction(s)
    }
}

func (b *RouteBrain) applySubaction(s simple.Subaction) {
    if s.Source == simple.NoneLocation ||
        s.Dest == simple.NoneLocation ||
        s.Source.Type == simple.NoneLocationType ||
        s.Dest.Type == simple.NoneLocationType ||
        s.Piece == (simple.Piece{}) {
        panic(fmt.Sprintf("Bot attempted to apply invalid Subaction: %+v", s))
    }
    b.table.ApplySubaction(s, b.identity)
}

func (b *RouteBrain) debugf(msg string, fargs ...interface{}) {
    log.Debug(fmt.Sprintf("(G%d) (Bot%s) (P%d) %s", b.gameId, b.identity, b.player, msg), fargs...)
}

func (b *RouteBrain) errorf(msg string, fargs ...interface{}) {
    log.Error(fmt.Sprintf("(G%d) (Bot%s) (P%d) %s", b.gameId, b.identity, b.player, msg), fargs...)
}

func max(x, y int) int {
    if x > y {
        return x
    }
    return y
}

func maxFloat(x, y float64) float64 {
    if x > y {
        return x
    }
    return y
}

func (b *RouteBrain) serializeTable() string {
    bytes, err := json.Marshal(b.table)
    if err != nil {
        panic(fmt.Sprintf("RouteBrain: serializeTable() error marshalling: '%s' b.table: %+v", err, b.table))
    }
    return string(bytes)
}

func min (x, y int) int {
    if x < y {
        return x
    }
    return y
}

