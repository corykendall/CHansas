package client

import (
    "encoding/json"
    "fmt"
	"github.com/gorilla/websocket"
    "local/hansa/log"
    "local/hansa/message"
    "local/hansa/simple"
)

type WebClient struct {
    identity simple.Identity
    c *websocket.Conn
    to chan message.Server
    from chan message.Client
    ipc simple.IpChecker
    ip string
}

func NewWebClient(i simple.Identity, c *websocket.Conn, ipc simple.IpChecker, ip string) *WebClient {
    log.Debug("WebClient created for %s (%s)", ip, c.RemoteAddr())
    return &WebClient{
        identity: i,
        c: c,
        to: make(chan message.Server, 100),
        from: make(chan message.Client, 100),
        ipc: ipc,
        ip: ip,
    }

}

// Call with a goroutine to start.
func (c *WebClient) Run() {
    c.ipc.Add(c.identity, c.ip)
    go c.read()
    c.send()
}

func (c *WebClient) Send(m message.Server) {
    c.to <-m
}

func (c *WebClient) Read() chan message.Client {
    return c.from
}

// TODO: Currently this only releases the write goroutine.  It's assumed that
// you are caling this because the client went away, but in some new use cases
// (userhandler) you might want to tell the client to eff off.  In that case
// we should be .Close the underlying websocket.  We currently don't, and leak
// those goroutines until the user leaves the password reset or mail confirm
// page.
func (c *WebClient) Done() {
    c.ipc.Sub(c.identity, c.ip)
    close(c.to)
}

func (c *WebClient) Identity() simple.Identity {
    return c.identity
}

func (c *WebClient) send() {
    for m := range c.to {
        bytes, err := json.Marshal(m)
        if err != nil {
            c.errorf("Error marshalling, giving up: %s", err)
            continue
        }
        err = c.c.WriteMessage(websocket.TextMessage, bytes)
        if err != nil {
            c.warnf("Disconnect (send): %s", err)
            continue
        }
    }
}

func (c *WebClient) read() {
    for {
		_, bytes, err := c.c.ReadMessage()
		if err != nil {
            c.debugf("Disconnect (read) (%s, %s): %s", c.ip, c.c.RemoteAddr(), err)
            close(c.from)
            c.c.Close()
            return
		}
        msg, err := message.UnmarshalClient(bytes)
        if err != nil {
            c.debugf("(ClientError) Unable to unmarshal bytes: %s", err)
        } else {
            c.from <-msg
        }
    }
}

func (c *WebClient) debugf(msg string, fargs ...interface{}) {
    log.Debug(fmt.Sprintf("(W-%s) %s", c.identity.Id, msg), fargs...)
}
func (c *WebClient) warnf(msg string, fargs ...interface{}) {
    log.Warn(fmt.Sprintf("(W-%s) %s", c.identity.Id, msg), fargs...)
}
func (c *WebClient) errorf(msg string, fargs ...interface{}) {
    log.Error(fmt.Sprintf("(W-%s) %s", c.identity.Id, msg), fargs...)
}
