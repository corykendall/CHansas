package client

import (
    "local/hansa/message"
    "local/hansa/simple"
)

type EmptyClient struct {}

func (c EmptyClient) Run() {}
func (c EmptyClient) Send(message.Server) {}
func (p EmptyClient) Done() {}
func (p EmptyClient) Read() chan message.Client {
    return make(chan message.Client)
}
func (p EmptyClient) Identity() simple.Identity {
    return simple.EmptyIdentity
}
