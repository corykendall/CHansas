package bot

import (
    "local/hansa/simple"
)

type Plan struct {

    // This is provided during plan creation.
    RouteId int
    Goal Goal

    // This is values used to weigh the plan
    Length PlanLength
    Actions int
    LeftoverMoves int
    Fitness Fitness
    FitnessValue float64
    FitnessDescription string

    // Subactions is how to enact the plan.  Sometimes a plan involves bumping, which
    // means we need to play the bump (and paying for the bump), and then wait
    // for the bumped player to respond.  To know when to return a partial plan
    // and wait for the opponents bump to complete, plan creators must track
    // the index of the last bump payment within Subactions in "Bumps".  An
    // opponent's response to a bump can never invalidate a route plan, so we
    // will save the rest of the plan (post bump) for after the opponent
    // responds.
    Subactions []simple.Subaction
    Bumps []int
}

type PlanLength int
const (

    // This plan doesn't use all of our actions.
    ShortPlan PlanLength = iota

    // This plan uses exactly all of our actions.
    FullPlan

    // This plan uses all of our actions except the last one (used only for
    // clearing a route, it's assumes you can clear it next turn as a starting
    // action).
    AlmostPlan

    // This plan uses all of our actions and doesn't succeed.
    LongPlan

    // This plan doesn't use all of our actions but is also unable to succeed.
    // An example is trying to fill a route via placing with no pieces in stock
    // and supply.  These are thrown out.
    UncompletablePlan
)

var lengthNames = map[PlanLength]string {
    ShortPlan: "Short",
    FullPlan: "Full",
    AlmostPlan: "Almost",
    LongPlan:"Long",
    UncompletablePlan:"Uncompletable",
}
