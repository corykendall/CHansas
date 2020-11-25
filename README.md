# CHansas

Running https://chansas.com

I write garbage code as quickly as I can with no tests.  Read at your own risk.

# Pointers

* server/game/game.go has the gross copy-paste guts of what is legal
* server/bot/routebrain.go has the iteration of bot code running now
* server/simple/... has a bunch of simple objects defining Hansa (like what the board looks like in boarddata.go)
* server/message/... has the wire API for the UI and Bots (both speak the same API) start in servermessage.go for outgoing and clientmessage.go for incoming.
* a couple of vestigal odds and ends are lying around, this code was ripped from CPokers.com

# TODO
* Bots weigh bump options well
* Testing/Debugging/Logging Cleanup
* Bonus Tokens
* Hand Bot tuning and differentiation (back to multiple bots)
* Learning (probably hand-tuned Genetic Algo over weights, simulation harness).

