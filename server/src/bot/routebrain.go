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
    weights Weights

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
        return plans[i].Fitness > plans[j].Fitness
    })
    b.debugf("Considered %d potential plans", len(plans))
    b.debugf("Chose a %s plan to %s on route %d with score %d",
        lengthNames[plans[0].Length], goalNames[plans[0].Goal], plans[0].RouteId, plans[0].Score)
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
    if weight == 0 {
        return []Plan{}
    }

    // Note that the subactions in each of these plans are not currently
    // applied to b.table.
    shape := simple.NoneShape
    if a == simple.CoellenAward {
        shape = simple.DiscShape
    }

    ps := b.generatePointsPlans(r, c, shape)
    for i, _ := range ps {
        ps[i].Goal = AwardGoal

        // We don't care as much about low scoring plays, because we are
        // getting a lot from the award.  Let's undo the PointsPlan penalty
        if ps[i].Score == 0 {
            ps[i].Fitness *= 10
        } else if ps[i].Score == 1 {
            ps[i].Fitness += 20
        }
        ps[i].Fitness += weight

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

// Returns the value of taking an office in a city, and what shape it is.  If
// you can't take this office without another upgrade (no priviledge, full
// office) returns 0.  This will return a value > 0 even if it will take many
// turns or movement to take the office.
func (b *RouteBrain) calcOfficeValue(city simple.City, c Context) (simple.Location, int, simple.Shape) {
    myPriviledge := b.table.PlayerBoards[b.player].GetPriviledge()

    presence := map[simple.PlayerColor]int{}
    presenceTiebreak := map[simple.PlayerColor]int{}
    shape := simple.CubeShape
    var location simple.Location

    for i, o := range city.Offices {
        if o.Piece == (simple.Piece{}) {
            if myPriviledge < o.Priviledge {
                return location, 0, simple.NoneShape
            }
            if o.Shape == simple.DiscShape {
                shape = simple.DiscShape
            }
            location = simple.Location{
                Type: simple.CityLocationType,
                Id: city.Id,
                Index: i,
            }
            break
        } else {
            presence[o.Piece.PlayerColor] += 1
            presenceTiebreak[o.Piece.PlayerColor] = i
        }
    }
    if location == simple.NoneLocation {
        return location, 0, simple.NoneShape
    }
    for _, p := range city.VirtualOffices {
        if p == (simple.Piece{}) {
            break
        }
        presence[p.PlayerColor] += 1
    }

    // This is the value we are ultimately returning.
    fitness := b.weights.OfficeLike[c.GameTime]
    if city.Award != simple.NoneAward {
        fitness += b.weights.AwardOfficeLike[c.GameTime]
    }

    myControl := 0
    winningControl := 0
    winningTie := false
    iWinTie := false
    for color, v := range presence {
        if color == b.color {
            myControl = v
        }
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
    if winningControl == 0 {
        fitness += b.weights.FirstOfficeLike
    } else if myControl == winningControl - 1 || (myControl == winningControl && winningTie && !iWinTie) {
        // This is the default case; we win control with this build.
    } else {
        fitness -= b.weights.NonControlOfficeAversion
    }

    if shape == simple.DiscShape {
        fitness -= b.weights.DiscOfficeAversion[c.GameTime]
    }

    // This handles a city not in our network (no bonus) adding a city to an
    // existing network (+1 bonus) or connecting 2 disparate networks (+n
    // bonus) with a multiplication.  This should make the reward for
    // connecting 2 moderate sized networks high enough, I think.
    networkDiff := b.table.Board.GetNetworkScoreIfCity(b.color, city.Id) -
        b.table.Board.GetNetworkScore(b.color)
    fitness += b.weights.NetworkLike[c.GameTime] * networkDiff

    return location, fitness, shape
}

func (b *RouteBrain) generateOfficePlans(r int, c Context) []Plan {
    haveDisc := c.SupplyDisc || c.StockDisc
    location, raw, shape := b.calcOfficeValue(b.table.Board.Cities[b.table.Board.Routes[r].LeftCityId], c)

    // This doesn't include the relative cost of a disc (stock vs supply when I have other pieces?)
    if raw == 0 || (shape == simple.DiscShape && !haveDisc) {
        location, raw, shape = b.calcOfficeValue(b.table.Board.Cities[b.table.Board.Routes[r].RightCityId], c)
    } else {
        lR, rR, sR := b.calcOfficeValue(b.table.Board.Cities[b.table.Board.Routes[r].RightCityId], c)
        if rR > raw && (sR != simple.DiscShape || haveDisc) {
            location, raw, shape = lR, rR, sR
        }
    }
    if raw == 0 || (shape == simple.DiscShape && !haveDisc) {
        return []Plan{}
    }

    // Note that the subactions in each of these plans are not currently
    // applied to b.table.
    ps := b.generatePointsPlans(r, c, shape)
    for i, _ := range ps {
        ps[i].Goal = OfficeGoal

        // We don't care as much about low scoring plays, because we are
        // getting a lot from the office.  Let's undo the PointsPlan penalty
        if ps[i].Score == 0 {
            ps[i].Fitness *= 10
        } else if ps[i].Score == 1 {
            ps[i].Fitness += 20
        }
        ps[i].Fitness += raw

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

            // Note there is a rare edge case here where the cleared route is
            // all discs and we want to take a cube office.  In that case we
            // just bomb out for now.
            if !handled {
                return []Plan{}
            }
        }
    }
    return ps
}

// Unlike the other generate*Plan methods, this is guaranteed to return a plan
// when shape is NoneShape. (no matter how bad, even if it generates 0 points).
// This way in degenerate board states where there is somehow no awards to
// claim, no offices I'm elligible for, and no other players to block (???) we
// still have a plan, even if there is no way to score points.
func (b *RouteBrain) generatePointsPlans(r int, c Context, shape simple.Shape) []Plan {

    // Each route spot will need to be filled via place or bump.
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
            } else {
                cubeToBump = append(cubeToBump, l)
            }
        }
    }

    // Before we start mutating anything with this plan, let's calculate
    // scoring.
    score := 0
    if (b.table.Board.Cities[b.table.Board.Routes[r].LeftCityId].GetControl() == b.color) {
        score++
    }
    if (b.table.Board.Cities[b.table.Board.Routes[r].RightCityId].GetControl() == b.color) {
        score++
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

    // Virtually every number here could be a weight.
    fitness := 100
    length := LongPlan
    if actionsLeft > 0 {
        if complete {
            length = ShortPlan
            fitness += 20
        } else {
            length = UncompletablePlan
            fitness -= 90
        }
    } else if routeCleared {
        length = FullPlan
        fitness += 15
    } else if routeFilled {
        length = AlmostPlan
        fitness += 5
    }

    actionsUsed := c.ActionsLeft - actionsLeft
    if actionsUsed == 1 {
        // This means all of our pieces were already on the route, we just had to clear.
        fitness += 50
    }

    fitness -= len(discToBump)*(b.weights.BumpAversion*2)
    fitness -= len(cubeToBump)*(b.weights.BumpAversion)

    // Changes based on score are the final tweaks made to fitness.  These will
    // be undone if we are being called by generateAwardPlans or
    // generateOfficePlans.
    if (score == 0) {
        fitness /= 10
    } else if (score == 2) {
        fitness += 50
    }

    b.table.UndoSubactions(subactions)
    return []Plan{Plan{
        RouteId: r,
        Goal: PointsGoal,
        Length: length,
        Actions: c.ActionsLeft - actionsLeft,
        Moves: false,
        Score: score,
        Fitness: fitness,
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
    // We don't like blocking if we have less than 5 free pieces (we have a lot
    // of pieces on the route and will lose bump potential), or if we have
    // nothing in stock (the goal is to get bumped)
    if c.LivePieces - c.BoardPieces < 5 ||
        (b.myStock(b.myDisc()) == simple.NoneLocation && b.myStock(b.myCube()) == simple.NoneLocation) {
        return []Plan{}
    }

    // We can only block if we are not on the route, and if someone else is on
    // the route, and if there is 1+ open spot.
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
            someoneElse = true
        }
    }
    if !someoneElse || len(openSpots) == 0 {
        return []Plan{}
    }

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

    // TODO: This is a simplification.  For now we add in how much we would
    // like to get a reward or an office on this route if we could, and use
    // that for how much we like to block someone else.
    fitness := b.weights.BlockLike
    fitness += max(b.rawAwardWeight(b.getRouteAward(r), c), b.weights.OfficeLike[c.GameTime])
    if length == LongPlan {
        fitness = 0
    } else if length == ShortPlan {
        fitness += 20
    }

    b.table.UndoSubactions(subactions)
    return []Plan{Plan{
        RouteId: r,
        Goal: BlockGoal,
        Length: length,
        Actions: c.ActionsLeft - actionsLeft,
        Moves: false,
        Score: 0,
        Fitness: fitness,
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
func (b *RouteBrain) rawAwardWeight(a simple.Award, c Context) int {
    if a == simple.NoneAward {
        return 0
    }
    if a == simple.CoellenAward {
        if !(c.StockDisc || c.SupplyDisc || c.BoardDisc) || b.getCoellenTarget() == simple.NoneLocation {
            return 0
        }
        return b.weights.Awards[simple.CoellenAward][int(c.GameTime)]
    }
    track := b.table.PlayerBoards[b.player].AwardTrackRemaining(a)
    if track == 0 {
        return 0
    }
    return b.weights.Awards[a][track-1]
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

func (b *RouteBrain) serializeTable() string {
    bytes, err := json.Marshal(b.table)
    if err != nil {
        panic(fmt.Sprintf("RouteBrain: serializeTable() error marshalling: '%s' b.table: %+v", err, b.table))
    }
    return string(bytes)
}

