package bot

import (
    "fmt"
    "local/hansa/simple"
)

type Fitness interface {
    Calculation(Weights) string
    Value(Weights) float64
}

type PointsPlanFitness struct {
    Length PlanLength
    BumpInfos []BumpFitnessInfo
    MyPoints int
    OthersPoints int
}

type AwardPlanFitness struct {
    Length PlanLength
    Award simple.Award
    AwardsLeft int
    BumpInfos []BumpFitnessInfo
    MyPoints int
    OthersPoints int
}

type OfficePlanFitness struct {
    Length PlanLength
    FirstOffice bool
    AwardOffice bool
    NonControlOffice bool
    DiscOffice bool
    NetworkDelta int
    BumpInfos []BumpFitnessInfo
    MyPoints int
    OthersPoints int
}

type BlockPlanFitness struct {
    Length PlanLength
    Discs int
    StockAndSupply int // Capped at of 10
    OpponentDesire float64
    DoublePiece bool
    DoublePlayer bool
}

type BumpFitnessInfo struct {
    Disc bool
    Bags int
    StockAndSupply int // Capped at of 10
}

func (a OfficePlanFitness) Calculation(w Weights) string {
    calc, _ := a.calculate(w)
    return calc
}

func (a OfficePlanFitness) Value(w Weights) float64 {
    _, value := a.calculate(w)
    return value
}

func (a OfficePlanFitness) calculate(w Weights) (string, float64) {
    calc := "%s = 100"
    value := 100.0

    calc = fmt.Sprintf("%s * %.2f (Length[%s])", calc, w.Length[a.Length], lengthNames[a.Length])
    value *= w.Length[a.Length]
    calc = fmt.Sprintf("%s * %.2f (Office)", calc, w.Office)
    value *= w.Office

    if a.FirstOffice {
        calc = fmt.Sprintf("%s * %.2f (FirstOffice)", calc, w.FirstOffice)
        value *= w.FirstOffice
    }
    if a.AwardOffice {
        calc = fmt.Sprintf("%s * %.2f (AwardOffice)", calc, w.AwardOffice)
        value *= w.AwardOffice
    }
    if a.NonControlOffice {
        calc = fmt.Sprintf("%s * %.2f (NonControlOffice)", calc, w.NonControlOffice)
        value *= w.NonControlOffice
    }
    if a.DiscOffice {
        calc = fmt.Sprintf("%s * %.2f (DiscOffice)", calc, w.DiscOffice)
        value *= w.DiscOffice
    }

    calc = fmt.Sprintf("%s * %.2f (NetworkDelta[%d])", calc, w.Network[a.NetworkDelta], a.NetworkDelta)
    value *= w.Network[a.NetworkDelta]

    for _, b := range a.BumpInfos {
        if b.Disc {
            calc = fmt.Sprintf("%s * %.2f (DiscBump)", calc, w.DiscBump)
            value *= w.DiscBump
        }
        calc = fmt.Sprintf("%s * %.2f (Bump[%d][%d])", calc,
            w.Bump[b.Bags][b.StockAndSupply], b.Bags, b.StockAndSupply)
        value *= w.Bump[b.Bags][b.StockAndSupply]
    }

    for i:=0;i<a.MyPoints;i++ {
        calc = fmt.Sprintf("%s * %.2f (MyPoints)", calc, w.MyPoints)
        value *= w.MyPoints
    }
    for i:=0;i<a.OthersPoints;i++ {
        calc = fmt.Sprintf("%s * %.2f (OthersPoints)", calc, w.OthersPoints)
        value *= w.OthersPoints
    }

    return calc, value
}

func (a AwardPlanFitness) Calculation(w Weights) string {
    calc, _ := a.calculate(w)
    return calc
}

func (a AwardPlanFitness) Value(w Weights) float64 {
    _, value := a.calculate(w)
    return value
}

func (a AwardPlanFitness) calculate(w Weights) (string, float64) {
    calc := "%s = 100"
    value := 100.0

    calc = fmt.Sprintf("%s * %.2f (Length[%s])", calc, w.Length[a.Length], lengthNames[a.Length])
    value *= w.Length[a.Length]

    calc = fmt.Sprintf("%s * %.2f (Awards[%d][%d])", calc,
        w.Awards[a.Award][a.AwardsLeft], a.Award, a.AwardsLeft)
    value *= w.Awards[a.Award][a.AwardsLeft]

    for _, b := range a.BumpInfos {
        if b.Disc {
            calc = fmt.Sprintf("%s * %.2f (DiscBump)", calc, w.DiscBump)
            value *= w.DiscBump
        }
        calc = fmt.Sprintf("%s * %.2f (Bump[%d][%d])", calc,
            w.Bump[b.Bags][b.StockAndSupply], b.Bags, b.StockAndSupply)
        value *= w.Bump[b.Bags][b.StockAndSupply]
    }

    for i:=0;i<a.MyPoints;i++ {
        calc = fmt.Sprintf("%s * %.2f (MyPoints)", calc, w.MyPoints)
        value *= w.MyPoints
    }
    for i:=0;i<a.OthersPoints;i++ {
        calc = fmt.Sprintf("%s * %.2f (OthersPoints)", calc, w.OthersPoints)
        value *= w.OthersPoints
    }

    return calc, value
}

func (a PointsPlanFitness) Calculation(w Weights) string {
    calc, _ := a.calculate(w)
    return calc
}

func (a PointsPlanFitness) Value(w Weights) float64 {
    _, value := a.calculate(w)
    return value
}

func (a PointsPlanFitness) calculate(w Weights) (string, float64) {
    calc := "%s = 100"
    value := 100.0

    calc = fmt.Sprintf("%s * %.2f (Length[%s])", calc, w.Length[a.Length], lengthNames[a.Length])
    value *= w.Length[a.Length]

    for _, b := range a.BumpInfos {
        if b.Disc {
            calc = fmt.Sprintf("%s * %.2f (DiscBump)", calc, w.DiscBump)
            value *= w.DiscBump
        }
        calc = fmt.Sprintf("%s * %.2f (Bump[%d][%d])", calc,
            w.Bump[b.Bags][b.StockAndSupply], b.Bags, b.StockAndSupply)
        value *= w.Bump[b.Bags][b.StockAndSupply]
    }

    for i:=0;i<a.MyPoints;i++ {
        calc = fmt.Sprintf("%s * %.2f (MyPoints)", calc, w.MyPoints)
        value *= w.MyPoints
    }
    for i:=0;i<a.OthersPoints;i++ {
        calc = fmt.Sprintf("%s * %.2f (OthersPoints)", calc, w.OthersPoints)
        value *= w.OthersPoints
    }

    return calc, value
}

func (a BlockPlanFitness) Calculation(w Weights) string {
    calc, _ := a.calculate(w)
    return calc
}

func (a BlockPlanFitness) Value(w Weights) float64 {
    _, value := a.calculate(w)
    return value
}

func (a BlockPlanFitness) calculate(w Weights) (string, float64) {
    calc := "%s = 100"
    value := 100.0

    calc = fmt.Sprintf("%s * %.2f (Length[%s])", calc, w.Length[a.Length], lengthNames[a.Length])
    value *= w.Length[a.Length]

    calc = fmt.Sprintf("%s * %.2f (OpponentDesire)", calc, a.OpponentDesire)
    value *= a.OpponentDesire

    if a.DoublePiece {
        calc = fmt.Sprintf("%s * %.2f (DoublePiece)", calc, w.DoublePieceBlock)
        value *= w.DoublePieceBlock
    }

    if a.DoublePlayer {
        calc = fmt.Sprintf("%s * %.2f (DoublePlayer)", calc, w.DoublePlayerBlock)
        value *= w.DoublePlayerBlock
    }

    // How much do you like to Block?  This depends on Your Disc level and the
    // pieces in your Stock+Supply (capped at 10).  This should have values for
    // [3][0-10], [5][0-10], [7][0-10], and [100][0-10].
    calc = fmt.Sprintf("%s * %.2f (Block[%d][%d])",
        calc, w.Block[a.Discs][a.StockAndSupply], a.Discs, a.StockAndSupply)
    value *= w.Block[a.Discs][a.StockAndSupply]

    return calc, value
}
