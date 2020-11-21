package bot

import (
    "local/hansa/simple"
)

type Weights struct {

    // Index 0 means "if there is 1 cube left on the track".
    // Index 1 means "if there are 2 cubes left on the track".
    // Coellen is whether it's early (0), mid (1), or late (2) game.
    // Keys is a base value, but is further modified by gametime and network.
    Awards map[simple.Award][]int
    
    OfficeLike map[GameTime]int
    AwardOfficeLike map[GameTime]int
    NetworkLike map[GameTime]int

    // Getting the first office can be strong because beating it requires
    // priviledge and perhaps one else wants it.  If you are second in you are
    // vulnerable to OfficeSwap token, and more likely that another player is
    // using this city for his network and will pip you.
    FirstOfficeLike int
    NonControlOfficeAversion int
    DiscOfficeAversion map[GameTime]int

    PointsLike int
    GivePointsAversion int
    BumpAversion int
    BlockLike int
}

var coryWeights = Weights{
    Awards: map[simple.Award][]int{
        simple.DiscsAward:      []int{30, 20, 30},
        simple.PriviledgeAward: []int{20, 20, 30},
        simple.BagsAward:       []int{40, 20, 50},
        simple.CoellenAward:    []int{10, 30, 70},
        simple.ActionsAward:    []int{70, 20, 60, 20, 90},
        simple.KeysAward:       []int{50, 20, 50, 80},
    },
    OfficeLike: map[GameTime]int{
        EarlyGame: 50,
        MidGame: 70,
        LateGame: 70,
    },
    AwardOfficeLike: map[GameTime]int{
        EarlyGame: 10,
        MidGame: 20,
        LateGame: 20,
    },
    NetworkLike: map[GameTime]int{
        EarlyGame: 10,
        MidGame: 10,
        LateGame: 20,
    },
    FirstOfficeLike: 10,
    NonControlOfficeAversion: 10,
    DiscOfficeAversion: map[GameTime]int{
        EarlyGame: 20,
        MidGame: 5,
        LateGame: 0,
    },
    PointsLike: 80,
    GivePointsAversion: 30,
    BumpAversion: 20,
    BlockLike: 50,
}

var jacobWeights = Weights{
    Awards: map[simple.Award][]int{
        simple.DiscsAward:      []int{30, 20, 30},
        simple.PriviledgeAward: []int{20, 20, 30},
        simple.BagsAward:       []int{40, 20, 50},
        simple.CoellenAward:    []int{10, 30, 70},
        simple.ActionsAward:    []int{40, 20, 60, 20, 90},
        simple.KeysAward:       []int{50, 20, 50, 80},
    },
    OfficeLike: map[GameTime]int{
        EarlyGame: 30,
        MidGame: 50,
        LateGame: 70,
    },
    AwardOfficeLike: map[GameTime]int{
        EarlyGame: 10,
        MidGame: 20,
        LateGame: 20,
    },
    NetworkLike: map[GameTime]int{
        EarlyGame: 15,
        MidGame: 15,
        LateGame: 30,
    },
    FirstOfficeLike: 10,
    NonControlOfficeAversion: 5,
    DiscOfficeAversion: map[GameTime]int{
        EarlyGame: 10,
        MidGame: 5,
        LateGame: 10,
    },
    PointsLike: 40,
    GivePointsAversion: 60,
    BumpAversion: 20,
    BlockLike: 100,
}

var caniceWeights = Weights{
    Awards: map[simple.Award][]int{
        simple.DiscsAward:      []int{40, 30, 30},
        simple.PriviledgeAward: []int{20, 20, 30},
        simple.BagsAward:       []int{40, 20, 50},
        simple.CoellenAward:    []int{10, 30, 70},
        simple.ActionsAward:    []int{60, 20, 60, 20, 90},
        simple.KeysAward:       []int{50, 20, 50, 80},
    },
    OfficeLike: map[GameTime]int{
        EarlyGame: 50,
        MidGame: 50,
        LateGame: 70,
    },
    AwardOfficeLike: map[GameTime]int{
        EarlyGame: 20,
        MidGame: 20,
        LateGame: 20,
    },
    NetworkLike: map[GameTime]int{
        EarlyGame: 30,
        MidGame: 40,
        LateGame: 30,
    },
    FirstOfficeLike: 10,
    NonControlOfficeAversion: 5,
    DiscOfficeAversion: map[GameTime]int{
        EarlyGame: 10,
        MidGame: 5,
        LateGame: 0,
    },
    PointsLike: 80,
    GivePointsAversion: 60,
    BumpAversion: 10,
    BlockLike: 30,
}
var bcripeWeights = Weights{
    Awards: map[simple.Award][]int{
        simple.DiscsAward:      []int{60, 50, 20},
        simple.PriviledgeAward: []int{40, 30, 30},
        simple.BagsAward:       []int{70, 55, 50},
        simple.CoellenAward:    []int{30, 30, 90},
        simple.ActionsAward:    []int{90, 60, 40, 20, 90},
        simple.KeysAward:       []int{60, 20, 50, 80},
    },
    OfficeLike: map[GameTime]int{
        EarlyGame: 10,
        MidGame: 20,
        LateGame: 90,
    },
    AwardOfficeLike: map[GameTime]int{
        EarlyGame: 20,
        MidGame: 20,
        LateGame: 0,
    },
    NetworkLike: map[GameTime]int{
        EarlyGame: 30,
        MidGame: 40,
        LateGame: 30,
    },
    FirstOfficeLike: 20,
    NonControlOfficeAversion: 5,
    DiscOfficeAversion: map[GameTime]int{
        EarlyGame: 0,
        MidGame: 0,
        LateGame: 0,
    },
    PointsLike: 90,
    GivePointsAversion: 20,
    BumpAversion: 0,
    BlockLike: 30,
}
var derekWeights = Weights{
    Awards: map[simple.Award][]int{
        simple.DiscsAward:      []int{60, 50, 20},
        simple.PriviledgeAward: []int{40, 30, 30},
        simple.BagsAward:       []int{70, 55, 50},
        simple.CoellenAward:    []int{30, 50, 110},
        simple.ActionsAward:    []int{50, 20, 40, 20, 90},
        simple.KeysAward:       []int{50, 20, 50, 80},
    },
    OfficeLike: map[GameTime]int{
        EarlyGame: 70,
        MidGame: 50,
        LateGame: 70,
    },
    AwardOfficeLike: map[GameTime]int{
        EarlyGame: 10,
        MidGame: 20,
        LateGame: 20,
    },
    NetworkLike: map[GameTime]int{
        EarlyGame: 30,
        MidGame: 40,
        LateGame: 30,
    },
    FirstOfficeLike: 20,
    NonControlOfficeAversion: 5,
    DiscOfficeAversion: map[GameTime]int{
        EarlyGame: 0,
        MidGame: 0,
        LateGame: 0,
    },
    PointsLike: 90,
    GivePointsAversion: 20,
    BumpAversion: 0,
    BlockLike: 30,
}
