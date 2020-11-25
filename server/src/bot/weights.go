package bot

import (
    "local/hansa/simple"
)

// A brain applies different weights to actions depending on what GameTime it
// is.
type WeightSet map[GameTime]Weights

// A subset of weights is used when evaluating the fitness of a plan based on
// the Goal.  So calculate fitness, we just take applicable weights and
// multiply everything with a base value of 100.
type Weights struct {

    //
    // All Goals
    //

    // Short plans are good because you can accomplish more than one in a
    // single turn. For example clearing a route when all of the pieces are
    // already on the route (1 action).  Long plans can't be discounted too
    // much, especially in the early game (for example, taking any reward is a
    // long plan).
    Length map[PlanLength]float64

    // Moving a piece can be a good thing (you were never going to clear the
    // route the piece was on and it wasn't blocking anything, and by moving
    // you are able to do more faster), or a bad thing (you are moving a useful
    // piece to a slightly more useful place).  It can also be good because you
    // are able to complete a goal with moves left over to work on other goals
    // (if you level up books to have 5 moves, for example).  To calculate the
    // effect of the move, fitness calculations will round up a value between
    // -200 and 200 to the nearest int key they can find in this map, and use
    // the found coefficient.  The value is the impact that the move has;
    // moving a peice from a "20" route to a "30" route should score "10" for
    // example.  For this map, you might have -100:0.5, 0:0.7, 50:0.9, 80:1.0,
    // 120:1.2, 150:1.5, 200: 1.8.There must be a value at 200, which is the
    // last value Fitness will look for.
    Move map[int]float64

    //
    // All Goals that Clear (not Block)
    //

    // How much you like to Bump Other players.  Note that this is in comparing
    // to _not_ bumping, so all of these should generally be sub 1.  How much
    // you dislike having to bump is dependent on your bags upgrade level and
    // the number of pieces you have in Stock+Supply, capped at 10 (as well as
    // on GameTime, as are all Weights at the WeightSet level).  This should
    // have values This should have values for [3][0-10], [5][0-10], [7][0-10],
    // and [100][0-10].  This coeffiecient may be applied multiple times (for
    // more than one bump in a single action.
    Bump map[int]map[int]float64

    // When you bump a disc, both DiscBump and Bump are used as coefficients.
    DiscBump float64

    // How valuable is it to get points when clearing a route.  Probably not
    // valuable early, and valuable late.  This may be applied multiple times
    // if multiple points are earned.
    MyPoints float64

    // How bad is it to give others points when clearing a route.
    OthersPoints float64

    //
    // Award Goals
    //

    // How important each award is, further indexed by which award you are
    // getting.  For example, the first upgrade to Actions is very important,
    // but the second (going from 3 -> 3 actions) is (maybe) very unimportant.
    // Index 0 means "if there is 1 cube left on the track".
    // Index 1 means "if there are 2 cubes left on the track".
    // For Coellen table, this is the index of the spot you can take (0 is 7
    // points, 3 is 11 points).
    Awards map[simple.Award][]float64

    //
    // Office Goals
    //

    // Raw value for how much I like offices.
    Office float64

    // Getting the first office can be strong because beating it requires
    // priviledge and perhaps no one else wants it, and we will score control.
    // If you are second in you are vulnerable to OfficeSwap token, and more
    // likely that another player will pip you.  Also you may be networking
    // where your opponent is networking, increasing contention.
    FirstOffice float64

    // Getting an award office is nice because others may give you points.
    // However it generally means passing on the award.
    AwardOffice float64

    // This is s map (to coeffiecent) from the delta to your endgame network
    // score if you build an office in this location, capped at 6.  If this is
    // 0, you have a network elsewhere that this build is not impacting.  If
    // you are building on an existing network and have a key, this will be 2.
    // If you are connecting 2 disjoint networks each of size 3, and you have a
    // key of 2, this will go from 6 score to 14 score, for a delta of 8.  This
    // value will be capped at 6 (assuming that is already a great play and
    // this map foesnt
    Network map[int]float64

    // If is generally bad to build an office which does not get you control,
    // however if it connects 2 disjoint networks perhaps it is worth it.
    NonControlOffice float64

    // It is possibly bad to use a disc (if you have only one), but it makes
    // your control harder to steal, and if you have more discs is perhaps not
    // so bad.
    DiscOffice float64

    //
    // Block Goals
    //

    // How much do you like to Block?  This depends on Your Disc level and the
    // pieces in your Stock+Supply (capped at 10).  This should have values for
    // [2][0-10], [3][0-10], [4][0-10], and [5][0-10].
    Block map[int]map[int]float64

    // How much less attractive is it to block when you have to use 2 pieces to
    // do so?
    DoublePieceBlock float64

    // How much less attractive is it to block when there are 2 other players
    // already holding some of the route?
    DoublePlayerBlock float64
}

var GenericWeightSet = WeightSet{
    EarlyGame: GenericEarlyWeights,
    MidGame: GenericMidWeights,
    LateGame: GenericLateWeights,
}

var GenericEarlyWeights = Weights{
    Length: map[PlanLength]float64 {
        ShortPlan: 1.1,
        FullPlan : 1.1,
        AlmostPlan: 1.1,
        LongPlan: 1.1,
        UncompletablePlan: 0.3,
    },
    Move: map[int]float64{
        -100: 0.5,
        0: 0.7,
        50: 0.9,
        80: 1.0,
        120: 1.2,
        150: 1.5,
        200: 1.8,
    },
    Bump: map[int]map[int]float64{
        3: map[int]float64{  0:0.0, 1:0.0, 2:0.8, 3:0.8, 4:0.8, 5:0.8, 6:0.8, 7:0.8, 8:0.8, 9:0.8, 10:0.8},
        5: map[int]float64{  0:0.0, 1:0.0, 2:0.8, 3:0.8, 4:0.8, 5:0.8, 6:0.8, 7:0.8, 8:0.8, 9:0.8, 10:0.8},
        7: map[int]float64{  0:0.0, 1:0.0, 2:0.8, 3:0.8, 4:0.8, 5:0.8, 6:0.8, 7:0.8, 8:0.8, 9:0.8, 10:0.8},
        100: map[int]float64{0:0.0, 1:0.0, 2:0.8, 3:0.8, 4:0.8, 5:0.8, 6:0.8, 7:0.8, 8:0.8, 9:0.8, 10:0.8},
    },
    DiscBump: 0.6,
    MyPoints: 1.3,
    OthersPoints: 0.7,
    Awards: map[simple.Award][]float64 {
        simple.DiscsAward: []float64{0.0, 1.5, 1.3, 2.5},
        simple.PriviledgeAward: []float64{0.0, 1.5, 1.3, 2.5},
        simple.BagsAward: []float64{0.0, 1.5, 1.2, 2.6},
        simple.CoellenAward: []float64{1.1, 1.2, 1.3, 2.5},
        simple.ActionsAward: []float64{0.0, 2.5, 1.1, 1.5, 1.1, 2.8},
        simple.KeysAward: []float64{0.0, 1.6, 1.0, 1.5, 2.2},
    },
    Office: 1.5,
    FirstOffice: 1.1,
    AwardOffice: 1.1,
    Network: map[int]float64 {
        0: 0.8,
        1: 1.05,
        2: 1.4,
        3: 1.6,
        4: 2.0,
        5: 2.5,
        6: 3.0,
    },
    NonControlOffice: 0.9,
    DiscOffice: 0.8,
    Block: map[int]map[int]float64{
        2: map[int]float64{0:0.0, 1:0.0, 2:0.0, 3:0.4, 4:0.6, 5:0.8, 6:1.0, 7:1.4, 8:1.4, 9:1.4, 10:1.8},
        3: map[int]float64{0:0.0, 1:0.0, 2:0.0, 3:0.4, 4:0.6, 5:0.8, 6:1.0, 7:1.4, 8:1.4, 9:1.4, 10:1.8},
        4: map[int]float64{0:0.0, 1:0.0, 2:0.0, 3:0.4, 4:0.6, 5:0.8, 6:1.0, 7:1.4, 8:1.4, 9:1.4, 10:1.8},
        5: map[int]float64{0:0.0, 1:0.0, 2:0.0, 3:0.4, 4:0.6, 5:0.8, 6:1.0, 7:1.4, 8:1.4, 9:1.4, 10:1.8},
    },
    DoublePieceBlock: 0.5,
    DoublePlayerBlock: 0.6,
}
var GenericMidWeights = Weights{
    Length: map[PlanLength]float64 {
        ShortPlan: 1.2,
        FullPlan : 1.1,
        AlmostPlan: 1.1,
        LongPlan: 1.1,
        UncompletablePlan: 0.3,
    },
    Move: map[int]float64{
        -100: 0.5,
        0: 0.7,
        50: 0.9,
        80: 1.0,
        120: 1.2,
        150: 1.5,
        200: 1.8,
    },
    Bump: map[int]map[int]float64{
        3: map[int]float64{  0:0.0, 1:0.0, 2:0.8, 3:0.8, 4:0.8, 5:0.8, 6:0.8, 7:0.8, 8:0.8, 9:0.8, 10:0.8},
        5: map[int]float64{  0:0.0, 1:0.0, 2:0.8, 3:0.8, 4:0.8, 5:0.8, 6:0.8, 7:0.8, 8:0.8, 9:0.8, 10:0.8},
        7: map[int]float64{  0:0.0, 1:0.0, 2:0.8, 3:0.8, 4:0.8, 5:0.8, 6:0.8, 7:0.8, 8:0.8, 9:0.8, 10:0.8},
        100: map[int]float64{0:0.0, 1:0.0, 2:0.8, 3:0.8, 4:0.8, 5:0.8, 6:0.8, 7:0.8, 8:0.8, 9:0.8, 10:0.8},
    },
    DiscBump: 0.6,
    MyPoints: 1.3,
    OthersPoints: 0.7,
    Awards: map[simple.Award][]float64 {
        simple.DiscsAward: []float64{0.0, 1.5, 1.3, 1.5},
        simple.PriviledgeAward: []float64{0.0, 1.5, 1.3, 1.5},
        simple.BagsAward: []float64{0.0, 1.5, 1.2, 1.6},
        simple.CoellenAward: []float64{1.1, 1.2, 1.3, 1.5},
        simple.ActionsAward: []float64{0.0, 2.5, 1.1, 1.5, 1.1, 1.8},
        simple.KeysAward: []float64{0.0, 1.6, 1.0, 1.5, 1.2},
    },
    Office: 1.5,
    FirstOffice: 1.1,
    AwardOffice: 1.1,
    Network: map[int]float64 {
        0: 0.8,
        1: 1.05,
        2: 1.4,
        3: 1.6,
        4: 2.0,
        5: 2.5,
        6: 3.0,
    },
    NonControlOffice: 0.9,
    DiscOffice: 0.8,
    Block: map[int]map[int]float64{
        2: map[int]float64{0:0.0, 1:0.0, 2:0.0, 3:0.4, 4:0.6, 5:0.8, 6:1.0, 7:1.4, 8:1.4, 9:1.4, 10:1.8},
        3: map[int]float64{0:0.0, 1:0.0, 2:0.0, 3:0.4, 4:0.6, 5:0.8, 6:1.0, 7:1.4, 8:1.4, 9:1.4, 10:1.8},
        4: map[int]float64{0:0.0, 1:0.0, 2:0.0, 3:0.4, 4:0.6, 5:0.8, 6:1.0, 7:1.4, 8:1.4, 9:1.4, 10:1.8},
        5: map[int]float64{0:0.0, 1:0.0, 2:0.0, 3:0.4, 4:0.6, 5:0.8, 6:1.0, 7:1.4, 8:1.4, 9:1.4, 10:1.8},
    },
    DoublePieceBlock: 0.5,
    DoublePlayerBlock: 0.6,
}
var GenericLateWeights = Weights{
    Length: map[PlanLength]float64 {
        ShortPlan: 1.2,
        FullPlan : 1.1,
        AlmostPlan: 1.1,
        LongPlan: 1.1,
        UncompletablePlan: 0.3,
    },
    Move: map[int]float64{
        -100: 0.5,
        0: 0.7,
        50: 0.9,
        80: 1.0,
        120: 1.2,
        150: 1.5,
        200: 1.8,
    },
    Bump: map[int]map[int]float64{
        3: map[int]float64{  0:0.0, 1:0.0, 2:0.8, 3:0.8, 4:0.8, 5:0.8, 6:0.8, 7:0.8, 8:0.8, 9:0.8, 10:0.8},
        5: map[int]float64{  0:0.0, 1:0.0, 2:0.8, 3:0.8, 4:0.8, 5:0.8, 6:0.8, 7:0.8, 8:0.8, 9:0.8, 10:0.8},
        7: map[int]float64{  0:0.0, 1:0.0, 2:0.8, 3:0.8, 4:0.8, 5:0.8, 6:0.8, 7:0.8, 8:0.8, 9:0.8, 10:0.8},
        100: map[int]float64{0:0.0, 1:0.0, 2:0.8, 3:0.8, 4:0.8, 5:0.8, 6:0.8, 7:0.8, 8:0.8, 9:0.8, 10:0.8},
    },
    DiscBump: 0.6,
    MyPoints: 1.7,
    OthersPoints: 0.5,
    Awards: map[simple.Award][]float64 {
        simple.DiscsAward: []float64{0.0, 1.5, 1.3, 1.5},
        simple.PriviledgeAward: []float64{0.0, 1.5, 1.3, 1.5},
        simple.BagsAward: []float64{0.0, 1.5, 1.2, 1.6},
        simple.CoellenAward: []float64{1.1, 1.2, 1.3, 1.5},
        simple.ActionsAward: []float64{0.0, 2.5, 1.1, 1.5, 1.1, 1.8},
        simple.KeysAward: []float64{0.0, 1.6, 1.0, 1.5, 1.2},
    },
    Office: 1.9,
    FirstOffice: 1.2,
    AwardOffice: 1.0,
    Network: map[int]float64 {
        0: 0.8,
        1: 1.15,
        2: 1.8,
        3: 2.0,
        4: 2.5,
        5: 4.0,
        6: 4.0,
    },
    NonControlOffice: 0.7,
    DiscOffice: 1.0,
    Block: map[int]map[int]float64{
        2: map[int]float64{0:0.0, 1:0.0, 2:0.0, 3:0.4, 4:0.6, 5:0.8, 6:1.0, 7:1.4, 8:1.4, 9:1.4, 10:1.8},
        3: map[int]float64{0:0.0, 1:0.0, 2:0.0, 3:0.4, 4:0.6, 5:0.8, 6:1.0, 7:1.4, 8:1.4, 9:1.4, 10:1.8},
        4: map[int]float64{0:0.0, 1:0.0, 2:0.0, 3:0.4, 4:0.6, 5:0.8, 6:1.0, 7:1.4, 8:1.4, 9:1.4, 10:1.8},
        5: map[int]float64{0:0.0, 1:0.0, 2:0.0, 3:0.4, 4:0.6, 5:0.8, 6:1.0, 7:1.4, 8:1.4, 9:1.4, 10:1.8},
    },
    DoublePieceBlock: 0.5,
    DoublePlayerBlock: 0.6,
}

var coryWeights = GenericWeightSet
var jacobWeights = GenericWeightSet
var caniceWeights = GenericWeightSet
var bcripeWeights = GenericWeightSet
var derekWeights = GenericWeightSet
