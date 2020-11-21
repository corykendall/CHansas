package server

import (
    "fmt"
    "sync"
    "local/hansa/simple"
    "local/hansa/log"
)

// These Ips never conflict
var whitelist []string = []string{}

// Implements simple.IpChecker.  When new connections are made and wrapped in a
// WebClient, they should Add() themselves.  When that client is done, it
// should Sub() itself.  When registering for a tournament or sitting down at a
// table, the tmt or table should Use() with the user, and respect the return
// (disallowing them if there is an ip collision).  When the tmt or table is
// done with the user (when the tournament is in seating is sufficient) it
// should DoneUse().
type IpChecker struct {
    mux sync.Mutex

    // map of identity -> ipaddress -> count of clients using that ipaddress.
    Ips map[simple.Identity]map[string]int

    // map of use case (like tmt123 or t4) to identites who are using that use
    // case with a slice of ips for each username.
    Usage map[string]map[simple.Identity][]string
}

func NewIpChecker() *IpChecker {
    return &IpChecker{
        Ips: map[simple.Identity]map[string]int{},
        Usage: map[string]map[simple.Identity][]string{},
        mux: sync.Mutex{},
    }
}

func (ipc *IpChecker) Add(i simple.Identity, a string) {
    ipc.mux.Lock()
    defer func() { ipc.mux.Unlock() }()
    log.Debug("IpChecker: Add(%s, %s)", i.Id, a)

    cs, ok := ipc.Ips[i]
    if !ok {
        cs = map[string]int{}
        ipc.Ips[i] = cs
    }

    c, ok := cs[a]
    if !ok {
        c = 0
    }

    log.Debug("IpChecker: %s (%s) (%d -> %d)", i.Id, a, c, c+1)
    cs[a] = c+1
}

func (ipc *IpChecker) Sub(i simple.Identity, a string) {
    ipc.mux.Lock()
    defer func() { ipc.mux.Unlock() }()
    log.Debug("IpChecker: Sub(%s, %s)", i.Id, a)

    cs, ok := ipc.Ips[i]
    if !ok {
        m := fmt.Sprintf("IpChecker: %s (%s) (0 -> -1) (no known ips)", i.Id, a)
        log.Fatal(m)
        panic(m)
    }

    c, ok := cs[a]
    if !ok {
        m := fmt.Sprintf("IpChecker: %s (%s) (0 -> -1) (other known ips)", i.Id, a)
        log.Fatal(m)
        panic(m)
    }

    log.Debug("IpChecker: %s (%s) (%d -> %d)", i.Id, a, c, c-1)
    if c > 1 {
        cs[a] = c-1
    } else {
        delete(cs, a)
    }
}

// Register 'identity' with the given usecase ('tmt123' or 't5' for example).
// If there are any other identities using this usecase with an IP that this
// identity is currently using, return the conflicting identity and false.
// Otherwise, return the EmptyIdentity and true.
func (ipc *IpChecker) Use(usecase string, identity simple.Identity) (simple.Identity, bool) {
    ipc.mux.Lock()
    defer func() { ipc.mux.Unlock() }()
    log.Debug("IpChecker: Use(%s, %s)", usecase, identity.Id)

    users, ok := ipc.Usage[usecase]
    if !ok {
        log.Debug("IpChecker: Usecase %s added", usecase)
        users = map[simple.Identity][]string{}
        ipc.Usage[usecase] = users
    }
    if _, ok := users[identity]; ok {
        m := fmt.Sprintf("IpChecker: %s is already using %s", identity.Id, usecase)
        log.Fatal(m)
        panic(m)
    }

    my, ok := ipc.Ips[identity];
    if !ok {
        log.Warn("IpChecker: No Ips found for %s, failing closed and "+
            "assuming they left while sending request", identity.Id)
        return simple.EmptyIdentity, false
    }

    myIps := []string{}
    for ip, _ := range my {
        myIps = append(myIps, ip)
    }

    if _, ok := match(myIps, whitelist); ok {
        log.Debug("IpChecker: Allow (in whitelist)")
    } else {
        for i, ips := range users {
            if ip, ok := match(myIps, ips); ok {
                log.Debug("IpChecker: IP Deny: usecase=%s, %s <> %s (%s)",
                    usecase, identity, i, ip)
                return i, false
            }
        }
    }

    users[identity] = myIps
    return simple.EmptyIdentity, true
}

func (ipc *IpChecker) DoneUse(usecase string, identity simple.Identity) {
    ipc.mux.Lock()
    defer func() { ipc.mux.Unlock() }()
    log.Debug("IpChecker: DoneUse(%s, %s)", usecase, identity.Id)

    users, ok := ipc.Usage[usecase]
    if !ok {
        log.Warn("IpChecker: %s Usecase does not exist; assuming hotload", usecase)
        return
    }

    _, ok = users[identity]
    if !ok {
        log.Warn("IpChecker: %s is not using %s; assuming hotload", identity.Id, usecase)
        return
    }

    delete(users, identity)
}

// Checks two list of ipaddresses for overlap.  If it finds one, returns the
// one it found and true.
func match(x []string, y []string) (string, bool) {
    for _, x1 := range x {
        for _, y1 := range y {
            if x1 == y1 {
                return x1, true
            }
        }
    }
    return "", false
}
