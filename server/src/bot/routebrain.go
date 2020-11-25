package bot

import (
    "encoding/json"
    "fmt"
    "sort"
    "strings"
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
    c := b.buildContext(actions)
    plans := []Plan{}
    b.debugf("choosePlan for %d actions with context %+v", actions, c)

    // First we generate plans without piece moves.  Generating a plan with a
    // PointsGoal and no pieces to move is guaranteed to succeed (though it may
    // be a very low scoring plan).
    routePointsPlans := map[int]Plan{}
    for i, _ := range b.table.Board.Routes {
        for _, g := range allGoals {
            p := b.generatePlan(i, g, c, []PieceScore{})
            if p.Goal == PointsGoal {
                routePointsPlans[i] = p
            }
            if p.Goal != NoneGoal {
                plans = append(plans, p)
            }
        }
    }

    // We use the no-move PointsPlans to decide on the relative value of each
    // piece on the board.
    piecesWorstToBest := b.scoreBoardPieces(routePointsPlans)

    // Now we are able to generate a version of every plan that prefers to move
    // pieces instead of placing them.
    if len(piecesWorstToBest) > 0 {
        for i, _ := range b.table.Board.Routes {
            for _, g := range allGoals {
                p := b.generatePlan(i, g, c, piecesWorstToBest)
                if p.Goal != NoneGoal {
                    plans = append(plans, p)
                }
            }
        }
    }

    // Let's sort all of the plans by their FitnessValue.  The best plan we
    // have found so far will be in the 0-index.
    sort.Slice(plans, func(i, j int) bool {
        return plans[i].FitnessValue > plans[j].FitnessValue
    })

    // Each of these plans may only have used a subset of possible move
    // subactions.  For example, you may need to move 2 pieces while clearing a
    // route, but be able to move 4 pieces per action through books upgrade.
    // We do another pass over these plans to make use of any leftover actions,
    // re-evaluating their Fitness scores appropriately.
    for i, _ := range plans {
        if plans[i].LeftoverMoves > 0 {
            b.useLeftoverMoves(plans, i, piecesWorstToBest, c)
        }
    }

    // Now that we have the plans fully fleshed out, let's resort them.  We may
    // have increased the Fitness Score of a plan by adding leftover piece
    // moves in a used action.
    sort.Slice(plans, func(i, j int) bool {
        return plans[i].FitnessValue > plans[j].FitnessValue
    })

    // We're done, but let's get some diagnostics before execute.
    allFitness := map[Goal][]float64{}
    for _, p := range plans {
        allFitness[p.Goal] = append(allFitness[p.Goal], p.FitnessValue)
    }
    str := fmt.Sprintf("Considered %d plans:", len(plans))
    for g, fs := range allFitness {
        max := 0.0
        for _, f := range fs {
            if f > max {
                max = f
            }
        }
        str = fmt.Sprintf("%s %s:{max:%.2f len:%d}", str, goalNames[g], max, len(fs))
    }
    b.debugf(str)

    parts := []string{}
    for _, p := range piecesWorstToBest {
        parts = append(parts, fmt.Sprintf("%d:%.2f", p.Location.Id, p.Score))
    }
    b.debugf("Board pieces valued at {%s}", strings.Join(parts, " "))
    b.debugf("Chose a %s plan to %s on route %d with fitness %.2f",
        lengthNames[plans[0].Length], goalNames[plans[0].Goal], plans[0].RouteId, plans[0].FitnessValue)
    b.debugf("Fitness calculation: %s", plans[0].FitnessDescription)
    if plans[0].LeftoverMoves > 0 {
        b.debugf("Plan used moves but didn't need all %d, so unrelated moves were added.",
            b.table.PlayerBoards[b.player].GetBooks())
    }
    b.debugf("Plan: %v", plans[0].Subactions)

    // This is where we mutate b.table chosen plan (we stop after applying bump
    // payment if it exists).
    for i, s := range plans[0].Subactions {
        b.applySubaction(s)
        if len(plans[0].Bumps) > 0 && plans[0].Bumps[0] == i {
            break
        }
    }

    return plans[0]
}

// This mutates plans to use any leftover moves.  This can happen if a Plan
// moves 2 pieces into a route to clear it, but has books 3.  The plan needs to
// use the last move in that action, but when calculating a route plan there is
// little knowledge of other routes.
func (b *RouteBrain) useLeftoverMoves(plans []Plan, index int, piecesWorstToBest []PieceScore, c Context) {
    routeId := plans[index].RouteId

    // We can only move pieces which weren't already moved to fulfill the given
    // plan, and which aren't on the route the given plan is working with.
    // This maintains worst to best sorting.
    myMovablePieces := []PieceScore{}
    for _, p := range piecesWorstToBest {
        alreadyMoved := false
        for _, s := range plans[index].Subactions {
            if s.Source == p.Location {
                alreadyMoved = true
                break
            }
        }
        if alreadyMoved {
            continue
        }
        if p.Location.Id != routeId {
            myMovablePieces = append(myMovablePieces, p)
        }
    }

    leftoverMoves := plans[index].LeftoverMoves
    ss := []simple.Subaction{}
    fitnessAdjustment := 0.0
    for i:=0; i<len(plans) && leftoverMoves>0 && len(myMovablePieces)>0;i++ {
        if i == index {
            continue
        }

        // TODO: I think this is the cause of the next bug: Jacob executes a
        // plan (move to block), has a move leftover, and then tries to do a
        // move which is really a bump.

        // This is a little shiesty; I'm saying any time another Plan's
        // subaction has a Dest on a route and the following subaction doesn't
        // have a Dest of Stock.  This way I'm sure it's not a bump, because
        // they would have to pay for their bump.
        moveTarget := simple.NoneLocation
        for _, s := range plans[i].Subactions {
            if moveTarget != simple.NoneLocation &&
                !(s.Dest.Type == simple.PlayerLocationType && s.Dest.Index == 5) {
                ss = append(ss, simple.Subaction{
                    Source: myMovablePieces[0].Location,
                    Dest: moveTarget,
                    Piece: myMovablePieces[0].Piece,
                })
                fitnessAdjustment += plans[i].FitnessValue/4.0
                fitnessAdjustment -= myMovablePieces[0].Score/4.0
                myMovablePieces = myMovablePieces[1:]
                leftoverMoves--
                moveTarget = simple.NoneLocation
                if leftoverMoves == 0 || len(myMovablePieces) == 0 {
                    break
                }
            }
            moveTarget = simple.NoneLocation

            // Note that we can't use leftoverMoves to "help" the current plan
            // by moving pieces to this route; the current plan wouldn't have
            // leftoverMoves if this was possible, and the current plan's
            // subactions aren't applied now so it may look possible.
            if s.Dest.Type == simple.RouteLocationType && s.Dest.Id != routeId {
                moveTarget = s.Dest
            }
        }
        if moveTarget != simple.NoneLocation {
            ss = append(ss, simple.Subaction{
                Source: myMovablePieces[0].Location,
                Dest: moveTarget,
                Piece: myMovablePieces[0].Piece,
            })
            myMovablePieces = myMovablePieces[1:]
            leftoverMoves--
        }
    }

    // Find the start of any move action, and we will insert our moves right
    // after it.
    moveIndex := -1
    for i, s := range plans[index].Subactions {
        if s.Source.Type == simple.RouteLocationType && s.Dest.Type == simple.RouteLocationType {
            moveIndex = i
            break
        }
    }
    if moveIndex == -1 {
        b.errorf("Plan has LeftoverMoves=%d but can't find Move start in %v",
            plans[index].LeftoverMoves, plans[index].Subactions)
        return
    }
    plans[index].Subactions = insertSubactions(plans[index].Subactions, moveIndex, ss...)

    // Update the fitness of our plan to include the fact that we're doing
    // these extra moves.
    if plans[index].Goal == AwardGoal {
        plans[index].Fitness.(*AwardPlanFitness).MovedScore += fitnessAdjustment
    } else if plans[index].Goal == OfficeGoal {
        plans[index].Fitness.(*OfficePlanFitness).MovedScore += fitnessAdjustment
    } else if plans[index].Goal == PointsGoal {
        plans[index].Fitness.(*PointsPlanFitness).MovedScore += fitnessAdjustment
    } else if plans[index].Goal == BlockGoal {
        plans[index].Fitness.(*BlockPlanFitness).MovedScore += fitnessAdjustment
    }
    plans[index].FitnessValue = plans[index].Fitness.Value(b.weights[c.GameTime])
    plans[index].FitnessDescription = plans[index].Fitness.Calculation(b.weights[c.GameTime])
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
func (b *RouteBrain) generatePlan(r int, g Goal, c Context, p []PieceScore) Plan {
    var ret Plan
    before := b.serializeTable()
    switch g {
        case AwardGoal:
            ret = b.generateAwardPlan(r, c, p)
        case OfficeGoal:
            ret = b.generateOfficePlan(r, c, p)
        case PointsGoal:
            ret = b.generatePointsPlan(r, c, simple.NoneShape, p)
        case BlockGoal:
            ret = b.generateBlockPlan(r, c, p)
    }
    after := b.serializeTable()
    if before != after {
        panic(fmt.Sprintf("RouteBrain mutated table! r=%d g=%d c=%v before=%s, after=%s",
            r, g, c, before, after))
    }
    return ret
}

func (b *RouteBrain) generateAwardPlan(r int, c Context, ps []PieceScore) Plan {
    a := b.getRouteAward(r)
    weight := b.rawAwardWeight(a, c)
    if weight < 0.0001 {
        return Plan{}
    }

    shape := simple.NoneShape
    if a == simple.CoellenAward {
        shape = simple.DiscShape
    }

    // Note that the subactions in each of these plans are not currently
    // applied to b.table.
    p := b.generatePointsPlan(r, c, shape, ps)
    if p.Goal == NoneGoal {
        return Plan{}
    }
    p.Goal = AwardGoal

    // Move over the pieces of PointsPlanFitness that we care about, and
    // recalculate our own fitness with the necessary fields.
    fitness := &AwardPlanFitness{
        Length: p.Fitness.(*PointsPlanFitness).Length,
        BumpInfos: p.Fitness.(*PointsPlanFitness).BumpInfos,
        MovedScore: p.Fitness.(*PointsPlanFitness).MovedScore,
        MyPoints: p.Fitness.(*PointsPlanFitness).MyPoints,
        OthersPoints: p.Fitness.(*PointsPlanFitness).OthersPoints,
    }
    fitness.Award = a
    if a == simple.CoellenAward {
        fitness.AwardsLeft = b.getCoellenTarget().Index
    } else {
        fitness.AwardsLeft = b.table.PlayerBoards[b.player].AwardTrackRemaining(a)
    }
    p.Fitness = fitness
    p.FitnessValue = fitness.Value(b.weights[c.GameTime])
    p.FitnessDescription = fitness.Calculation(b.weights[c.GameTime])

    // If we cleared, we also need to take our reward.
    if p.Length == ShortPlan || p.Length == FullPlan {

        // The coellen reward is very similar to taking a disc office.  We
        // look for a clear subaction with a disc (guaranteed because we
        // passed disc=true to generatePointsPlan) and replace the
        // destination with the coellen table.
        if a == simple.CoellenAward {
            for i2:=len(p.Subactions)-1;i2>=0;i2-- {
                candidate := p.Subactions[i2]
                if candidate.Dest.Type == simple.PlayerLocationType &&
                    candidate.Piece.Shape == simple.DiscShape {
                    p.Subactions[i2].Dest = b.getCoellenTarget()
                    break
                }
            }

        // All other rewards require us to move a new piece from a
        // playerboard area to the supply.  In order to know which
        // subindexes in our supply are open, we need to apply all previous
        // subactions.
        } else {
            b.applySubactions(p.Subactions)
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
            b.table.UndoSubactions(p.Subactions)
            p.Subactions = append(p.Subactions, s)
        }
    }
    return p
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

// This is used to decide what piece to move for a move action
// (and the negative impact of doing so).
type PieceScore struct {
    Piece simple.Piece
    Location simple.Location
    Score float64
}
func (b *RouteBrain) scoreBoardPieces(routePointsPlans map[int]Plan) []PieceScore {
    r := []PieceScore{}
    for ri, route := range b.table.Board.Routes {
        for pi, piece := range route.Spots {
            if piece.PlayerColor == b.color {
                r = append(r, PieceScore{
                    Piece: piece,
                    Location: simple.Location{
                        Type: simple.RouteLocationType,
                        Id: ri,
                        Index: pi,
                        Subindex: 0,
                    },
                    Score: 0.0,
                })
            }
        }
    }
    for ri, ps := range r {
        if p, ok := routePointsPlans[ps.Location.Id]; ok {
            r[ri].Score = p.FitnessValue
        } else {
            panic(fmt.Sprintf("Bot failed to generate PointsPlan for route '%d'", ps.Location.Id))
        }
    }
    sort.Slice(r, func(i, j int) bool {
        return r[i].Score < r[j].Score
    })
    return r
}

func (b *RouteBrain) generateOfficePlan(r int, c Context, p []PieceScore) Plan {
    plans := []Plan{}

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
        p := b.generatePointsPlan(r, c, shape, p)
        if p.Goal == NoneGoal {
            continue
        }
        p.Goal = OfficeGoal

        // Steal the parts of PointsPlanFitness that we want to use.
        fitness := &OfficePlanFitness{
            Length: p.Fitness.(*PointsPlanFitness).Length,
            BumpInfos: p.Fitness.(*PointsPlanFitness).BumpInfos,
            MovedScore: p.Fitness.(*PointsPlanFitness).MovedScore,
            MyPoints: p.Fitness.(*PointsPlanFitness).MyPoints,
            OthersPoints: p.Fitness.(*PointsPlanFitness).OthersPoints,
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
        p.Fitness = fitness
        p.FitnessValue = fitness.Value(b.weights[c.GameTime])
        p.FitnessDescription = fitness.Calculation(b.weights[c.GameTime])

        // If we cleared, we need to replace one of the clearing Subactions
        // with a move into our office.  If we took a disc office, we need to
        // move a disc, and if we took a cube office, we need to move a cube.
        if p.Length == ShortPlan || p.Length == FullPlan {

            // We are looking for a clear subaction with the right shape
            handled := false
            for i2:=len(p.Subactions)-1;i2>=0;i2-- {
                candidate := p.Subactions[i2]
                if candidate.Source.Type == simple.RouteLocationType &&
                    candidate.Source.Id == r &&
                    candidate.Piece.Shape == shape {
                    p.Subactions[i2].Dest = location
                    handled = true
                    break
                }
            }

            // TODO: Note there is a rare edge case here where the cleared route is
            // all discs and we want to take a cube office.  In that case we
            // just bomb out for now.
            if !handled {
                return Plan{}
            }
        }
        plans = append(plans, p)
    }

    if len(plans) == 0 {
        return Plan{}
    }
    if len(plans) == 1 {
        return plans[0]
    }
    if plans[0].FitnessValue > plans[1].FitnessValue {
        return plans[0]
    }
    return plans[1]
}

// Unlike the other generate*Plan methods, this is guaranteed to return a plan
// when shape is NoneShape and movablePieces is empty. (no matter how bad, even
// if it generates 0 points).  This way in degenerate board states where there
// is somehow no awards to claim, no offices I'm elligible for, no points to
// gain, and no other players to block (???) we still have a plan.
func (b *RouteBrain) generatePointsPlan(
    r int,
    c Context,
    shape simple.Shape,
    movablePieces []PieceScore,
) Plan {
    fitness := &PointsPlanFitness{}

    // Each route spot will need to be filled via place, move, or bump.  We set
    // 'shape' to be NoneShape as soon as we fulfill its requirement of being
    // in the route.   This may be if we find it already there, move it there,
    // or place it there.
    stockAndSupply := min(10, b.myStockAndSupplyCount())
    discToBump := []simple.Location{}
    cubeToBump := []simple.Location{}
    openSpot := []simple.Location{}
    for i, p := range b.table.Board.Routes[r].Spots {
        if p.PlayerColor == b.color {
            if p.Shape == shape {
                shape = simple.NoneShape
            }
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
    if len(movablePieces) > 0 && len(openSpot) == 0 {
        return Plan{}
    }

    // Before we start mutating anything with this plan, let's calculate how
    // many points clearing this route will give us and opponents.
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

    // A copy of movablePieces with a required piece sorted to the front.  If
    // we have some movablePieces but are missing a required piece or they are
    // all on our route, then we are done.
    myMovablePieces := []PieceScore{}
    for _, p := range movablePieces {
        if p.Location.Id != r {
            myMovablePieces = append(myMovablePieces, p)
        }
    }
    if shape != simple.NoneShape {
        for i, p := range myMovablePieces {
            if p.Piece.Shape == shape {
                myMovablePieces[0], myMovablePieces[i] = p, myMovablePieces[0]
                shape = simple.NoneShape
                break
            }
        }
    }
    if len(movablePieces) > 0 && (len(myMovablePieces) == 0 ||
        (shape != simple.NoneShape && shape != myMovablePieces[0].Piece.Shape)) {
        return Plan{}
    }

    // Fill the route with our pieces.  First move to open spots, then place in
    // open spots, then bump cubes, then bump discs.  If we need a shape on the
    // route for clearing reasons, move/place that piece first.
    actionsLeft := c.ActionsLeft
    leftoverMoves := 0
    complete := true
    subactions := []simple.Subaction{}
    bumps := []int{}
    if actionsLeft > 0 && len(movablePieces) > 0 {
        m := b.table.PlayerBoards[b.player].GetBooks()
        actionsLeft--
        for _, ps := range myMovablePieces {
            if len(openSpot) == 0 {
                break
            }
            if m == 0 {
                if actionsLeft == 0 {
                    complete = false
                    break
                } else {
                    actionsLeft--
                    m = b.table.PlayerBoards[b.player].GetBooks()
                }
            }
            sa := simple.Subaction{
                Source: ps.Location,
                Dest: openSpot[0],
                Piece: ps.Piece,
            }
            b.table.ApplySubaction(sa, b.identity)
            subactions = append(subactions, sa)
            openSpot = openSpot[1:]
            m--
            fitness.MovedScore -= ps.Score/4.0 // This /4 is meh.
        }
        leftoverMoves = m
    }
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
    return Plan{
        RouteId: r,
        Goal: PointsGoal,
        Length: length,
        Actions: c.ActionsLeft - actionsLeft,
        LeftoverMoves: leftoverMoves,
        Fitness: fitness,
        FitnessValue: fitness.Value(b.weights[c.GameTime]),
        FitnessDescription: fitness.Calculation(b.weights[c.GameTime]),
        Subactions: subactions,
        Bumps: bumps,
    }
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
func (b *RouteBrain) generateBlockPlan(r int, c Context, movablePieces []PieceScore) Plan {
    fitness := &BlockPlanFitness{
        Discs: b.table.PlayerBoards[b.player].GetBooks(),
        StockAndSupply: min(b.myStockAndSupplyCount(), 10),
    }

    // We can only block if we are not on the route, and if someone else is on
    // the route, and if there is 1+ open spot.  Note that the goal is to get
    // bumped; if we actually want the route, generatePointsPlan or some
    // derivative will score highly, and we will use that plan instead.
    someoneElse := simple.NonePlayerColor
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
            return Plan{}
        } else {
            if someoneElse != simple.NonePlayerColor && someoneElse != s.PlayerColor {
                fitness.DoublePlayer = true
            }
            someoneElse = s.PlayerColor
        }
    }
    if someoneElse == simple.NonePlayerColor || len(openSpots) == 0 {
        return Plan{}
    }
    fitness.DoublePiece = len(openSpots) > 1

    // Fill the open spots on the route with our pieces.  First move to open
    // spots, then place in open spots.  Note that we will always have the
    // pieces in our Supply+Stock because of our opening check in this
    // function.
    actionsLeft := c.ActionsLeft
    leftoverMoves := 0
    complete := true
    subactions := []simple.Subaction{}
    if actionsLeft > 0 && len(movablePieces) > 0 {
        m := b.table.PlayerBoards[b.player].GetBooks()
        actionsLeft--
        for _, ps := range movablePieces {
            if len(openSpots) == 0 {
                break
            }
            if m == 0 {
                if actionsLeft == 0 {
                    complete = false
                    break
                } else {
                    actionsLeft--
                    m = b.table.PlayerBoards[b.player].GetBooks()
                }
            }
            sa := simple.Subaction{
                Source: ps.Location,
                Dest: openSpots[0],
                Piece: ps.Piece,
            }
            b.table.ApplySubaction(sa, b.identity)
            subactions = append(subactions, sa)
            openSpots = openSpots[1:]
            m--
            fitness.MovedScore -= ps.Score/4.0 // This /4 is meh.
        }
        leftoverMoves = m
    }
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
    return Plan{
        RouteId: r,
        Goal: BlockGoal,
        Length: length,
        Actions: c.ActionsLeft - actionsLeft,
        LeftoverMoves: leftoverMoves,
        Fitness: fitness,
        FitnessValue: fitness.Value(b.weights[c.GameTime]),
        FitnessDescription: fitness.Calculation(b.weights[c.GameTime]),
        Subactions: subactions,
        Bumps: []int{},
    }
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

// Assume element type is int.
func insertSubactions(s []simple.Subaction, k int, vs ...simple.Subaction) []simple.Subaction {
	if n := len(s) + len(vs); n <= cap(s) {
		s2 := s[:n]
		copy(s2[k+len(vs):], s[k:])
		copy(s2[k:], vs)
		return s2
	}
	s2 := make([]simple.Subaction, len(s) + len(vs))
	copy(s2, s[:k])
	copy(s2[k:], vs)
	copy(s2[k+len(vs):], s[k:])
	return s2
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

func minFloat(x, y float64) float64 {
    if x < y {
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

