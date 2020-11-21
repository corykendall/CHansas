package bot

import (
    "local/hansa/message"
    "local/hansa/simple"
)

type Manager struct {}

func NewManager() *Manager {
    return &Manager{}
}

func (m *Manager) NewBot(i simple.Identity, gameId int) *Bot {
    /*
    var brain Brain
    if i.Id == "B5" || i.Id == "B1" {
        brain = &RouteBrain{identity: i, gameId: gameId, weights: botWeights[i.Id]}
    } else {
        brain = &PlaceBrain{identity: i, gameId: gameId}
    }
    */

    brain := &RouteBrain{identity: i, gameId: gameId, weights: botWeights[i.Id]}
    //brain := &PlaceBrain{identity: i, gameId: gameId}

    b := &Bot{
        i,
        brain,
        make(chan message.Server, 10),
        make(chan message.Client, 10),
    }
    go b.Run()
    return b
}

var botIdentities = map[string]simple.Identity{
    "B1": simple.NewBotIdentity("B1", "Derek (Bot)"),
    "B2": simple.NewBotIdentity("B2", "Canice (Bot)"),
    "B3": simple.NewBotIdentity("B3", "Jacob (Bot)"),
    "B4": simple.NewBotIdentity("B4", "BCripe (Bot)"),
    "B5": simple.NewBotIdentity("B5", "Cory (Bot)"),
}

var botWeights = map[string]Weights {
    "B1": coryWeights,
    "B2": coryWeights,
    "B3": coryWeights,
    "B4": coryWeights,
    "B5": coryWeights,
}

func (m *Manager) GetIdentity(id string) simple.Identity {
    if b, ok := botIdentities[id]; ok {
        return b
    }
    return simple.EmptyIdentity
}

