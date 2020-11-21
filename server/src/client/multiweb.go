package client

import (
    "fmt"
    "reflect"
    "sync"
    "local/hansa/log"
    "local/hansa/message"
    "local/hansa/simple"
)

type MultiWebClient struct {
    identity simple.Identity

    mux sync.Mutex
    c []*WebClient

    to chan message.Server
    from chan message.Client
    join chan Join 
}

type Join struct {
    c *WebClient
    d chan struct{}
}

func NewMultiWebClient(c *WebClient) *MultiWebClient {
    return &MultiWebClient{
        identity: c.Identity(),
        mux: sync.Mutex{},
        c: []*WebClient{c},
        to: make(chan message.Server, 100),
        from: make(chan message.Client, 100),
        join: make(chan Join, 2),
    }
}

// Used during hotdeploy to create a MultiWebClient with no underlying
// WebClients (but which is smart enough to consume WebClients when the user
// connect).
func NewDisconnectedMultiWebClient(i simple.Identity) *MultiWebClient {
    r := &MultiWebClient{
        identity: i,
        mux: sync.Mutex{},
        c: []*WebClient{},
        to: make(chan message.Server, 100),
        from: make(chan message.Client, 100),
        join: make(chan Join, 2),
    }

    // This is what normally happens when the last client leaves.
    close(r.from)

    return r
}

// Call with a goroutine to start.
func (mc *MultiWebClient) Run() {
    mc.tracef("Running")
    go mc.read()
    mc.send()
}

func (mc *MultiWebClient) Send(m message.Server) {
    mc.to <-m
}

func (mc *MultiWebClient) Read() chan message.Client {
    return mc.from
}

func (mc *MultiWebClient) Identity() simple.Identity {
    return mc.identity
}

func (mc *MultiWebClient) Consume(c *WebClient) {
    d := make(chan struct{})
    mc.join <-Join{c: c, d: d}
    <-d
}

func (mc *MultiWebClient) Done() {
    close(mc.join)
    close(mc.to)
    // read() closed from last client remove()
}

func (mc *MultiWebClient) send() {
    for m := range mc.to {
        for _, c := range mc.getc() {
            c.Send(m)
        }
    }
}

func (mc *MultiWebClient) read() {
    mc.tracef("read loop running")
    for {
        clients, cases := mc.gets()
        i, v, ok := reflect.Select(cases)
        mc.tracef("read: %d, %+v, %t ", i, v, ok)
        if !ok {
            if i == 0 {
                return
            } else {
                mc.tracef("read: removing")
                mc.remove(clients[i])
                mc.tracef("read: done remove")
            }
        } else {
            if i == 0 {
                j := v.Interface().(Join)
                mc.tracef("read: joining ")
                mc.add(j.c)
                mc.tracef("read: done joining")
                close(j.d)
            } else {
                mc.tracef("read: exposing message")
                mc.from <- v.Interface().(message.Client)
            }
        }
    }
}

func (mc *MultiWebClient) gets() ([]*WebClient, []reflect.SelectCase) {
    clients := []*WebClient{nil}
    cases := []reflect.SelectCase{reflect.SelectCase{
            Dir: reflect.SelectRecv,
            Chan: reflect.ValueOf(mc.join)}}
    for _, c := range mc.getc() {
        clients = append(clients, c)
        cases = append(cases, reflect.SelectCase{
            Dir: reflect.SelectRecv,
            Chan: reflect.ValueOf(c.Read())})
    }
    return clients, cases
}

func (mc *MultiWebClient) getc() []*WebClient {
    mc.mux.Lock()
    defer func() { mc.mux.Unlock() }()
    cp := []*WebClient{}
    for _, c := range mc.c {
        cp = append(cp, c)
    }
    return cp
}

func (mc *MultiWebClient) add(c *WebClient) {
    mc.mux.Lock()
    defer func() { mc.mux.Unlock() }()
    l := len(mc.c)
    mc.tracef("New Connection (%d -> %d)", l, l+1)
    mc.c = append(mc.c, c)
    if len(mc.c) == 1 {
        mc.from = make(chan message.Client, 100)
    }
}

func (mc *MultiWebClient) remove(r *WebClient) {
    mc.mux.Lock()
    defer func() { mc.mux.Unlock() }()
    for i, c := range mc.c {
        // pointer comparison
        if c == r {
            c.Done()
            l := len(mc.c)
            mc.tracef("Lost Connection (%d -> %d)", l, l-1)
            mc.c = append(mc.c[:i], mc.c[i+1:]...)
            if len(mc.c) == 0 {
                // This signals readers that there isn't an active connection.
                mc.tracef("Closing exposed read chan")
                close(mc.from)
            }
            return
        }
    }
}

func (mc *MultiWebClient) debugf(msg string, fargs ...interface{}) {
    log.Debug(fmt.Sprintf("(mwc) (%s) %s", mc.Identity().Id, msg), fargs...)
}

func (mc *MultiWebClient) tracef(msg string, fargs ...interface{}) {
    log.Trace(fmt.Sprintf("(mwc) (%s) %s", mc.Identity().Id, msg), fargs...)
}
