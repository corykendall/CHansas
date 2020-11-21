package client

import (
    "local/hansa/message"
    "local/hansa/simple"
)

type Client interface {
    Identity() simple.Identity
    Run()
    Send(message.Server)
    Read() chan message.Client
    Done() // Return immediately
}
