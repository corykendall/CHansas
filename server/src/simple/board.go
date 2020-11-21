package simple

type Board struct {
    Name string
    Cities []City
    Routes []Route
}

func (b Board) GetBonusRouteCompleted(color PlayerColor) bool {
    start := -1
    end := -1

    for _, c := range b.Cities {
        if c.BonusTerminus {
            if start == -1 {
                start = c.Id
            } else if end == -1 {
                end = c.Id
                break
            }
        }
    }
    if end == -1 {
        return false
    }
    if b.Cities[start].GetPresence(color) == 0 || b.Cities[end].GetPresence(color) == 0{
        return false
    }

    adds := 1
    oldFrontier := []City{b.Cities[start]}
    frontier := []City{}
    for adds > 0 {
        adds = 0
        for _, oc := range oldFrontier {
            for _, nc := range b.Cities {
                if containsCity(nc, oldFrontier) {
                    continue
                }
                if nc.GetPresence(color) == 0 {
                    continue
                }
                if b.adjacentCity(oc, nc) {
                    if nc.Id == end {
                        return true
                    }
                    adds++
                    frontier = append(frontier, nc)
                }
            }
        }
        oldFrontier = append(oldFrontier, frontier...)
        frontier = []City{}
    }
    return false
}

func (b *Board) GetFilledCityCount() int {
    r := 0
    for _, c := range b.Cities {
        filled := true
        for _, o := range c.Offices {
            if o.Piece == (Piece{}) {
                filled = false
                break
            }
        }
        if filled {
            r++
        }
    }
    return r
}

// Note this doesn't include key multiplier.
func (b *Board) GetNetworkScoreIfCity(color PlayerColor, c int) int {
    return b.getNetworkScore(func(c2 City) int {
        if c2.Id == c {
            return 1 + c2.GetPresence(color)
        }
        return c2.GetPresence(color)
    })
}

// Note this doesn't include key multiplier.
func (b *Board) GetNetworkScore(color PlayerColor) int {
    return b.getNetworkScore(func(c City) int {
        return c.GetPresence(color)
    })
}

// This takes a function to check for presence in a city.  It's factored this
// way so that bots can ask hypotheticals "what if I were in this city". 
func (b *Board) getNetworkScore(getPresence func(City) int) int {
    type Network struct {
        Cities map[int]struct{}
        Score int
    }
    networks := []Network{}

    for _, c := range b.Cities {
        p := getPresence(c)
        if p == 0 {
            continue
        }
        handled := false
        for _, n := range networks {
            if _, ok := n.Cities[c.Id]; ok {
                handled = true
                break
            }
        }
        if handled {
            continue
        }
        n := Network{
            Cities: map[int]struct{}{c.Id: struct{}{}},
            Score: p,
        }

        frontier := []int{c.Id}
        for ;len(frontier)>0; {
            newFrontier := []int{}
            for _, c2 := range frontier {
                for _, c3 := range b.Cities {
                    if _, ok := n.Cities[c3.Id]; ok {
                        continue
                    }
                    if p2 := getPresence(c3); p2 != 0 && b.adjacentCity(b.Cities[c2], c3) {
                        n.Score += p2
                        n.Cities[c3.Id] = struct{}{}
                        newFrontier = append(newFrontier, c3.Id)
                    }
                }
            }
            frontier = newFrontier
        }
        networks = append(networks, n)
    }

    max := 0
    for _, n := range networks {
        if n.Score > max {
            max = n.Score
        }
    }
    return max
}

func containsCity(c City, cs []City) bool {
    for _, c2 := range cs {
        if c.Id == c2.Id {
            return true
        }
    }
    return false
}

func (b *Board) adjacentCity(x, y City) bool {
    for _, r := range b.Routes {
        if (r.LeftCityId == x.Id && r.RightCityId == y.Id) ||
            (r.RightCityId == x.Id && r.LeftCityId == y.Id) {
            return true
        }
    }
    return false
}

/*
func (b Board) DeepEquals(that Board) bool {
    return b.Name == that.Name &&
        deepEqualsCities(b.Cities, that.Cities) &&
        deepEqualsRoutes(b.Routes, that.Routes)
}

func deepEqualsCities(this []Cities, that []Cities) bool {
    for i, c := range this {
        if !c.deepEquals(that[i]) {
            return false
        }
    }
    return true
}

*/
