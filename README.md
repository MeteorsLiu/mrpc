# Meteor RPC

## Design

Based in `Reactor` network model.

### Why  `Reactor`
In common Goroutine model, one connection spwan one goroutine with one stack space(4K per Goroutine).

When we are under very high preesure, for example, 1 million connections, 

that's said, it requires mininal `n*4096` bytes memory for stack spaces, 

which will cause memory shortage significantly.

To solve that problem without using stackless coroutine, `reactor` model is a good choice.


