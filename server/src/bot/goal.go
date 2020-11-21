package bot

type Goal int
const (
    NoneGoal Goal = iota
    AwardGoal 
    OfficeGoal
    PointsGoal
    BlockGoal
)
var allGoals = []Goal{
    AwardGoal,
    OfficeGoal,
    PointsGoal,
    BlockGoal,
}

var goalNames = map[Goal]string{
    NoneGoal: "None",
    AwardGoal: "Award",
    OfficeGoal: "Office",
    PointsGoal: "Points",
    BlockGoal: "Block",
}
    
