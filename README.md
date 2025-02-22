# Meteor RPC[WIP]

## Design

Based in `Reactor` network model.

### Why  `Reactor`
In common Goroutine model, one connection spwan one goroutine with one stack space(4K per Goroutine).

When we are under very high preesure, for example, 1 million connections, 

that's said, it requires mininal `n*4096` bytes memory for stack spaces, 

which will cause memory shortage significantly.

To solve that problem without using stackless coroutine, `reactor` model is a good choice.

It's almost impossible or very hard to implement stackless coroutine in Go.

Reason:
1. We have to consider about the place where the stack frame of coroutine stores in with GC
2. How to pause or resume coroutine? In C++, these actions are compiled to static machine code, but we can't.
3. We have to rewrite all network stack for stackless coroutine, that's really impossible.

### Shortage of `Reactor`
1. Single thread, bad concurrency
2. Still need to rewrite parts of network library in Go

First question is solved by `thread per core` model.
Second...No idea for now, so this project is still in progress, because rewriting is complicated and meaningless.


### Plan
I try to solve that problem in other ways like rebuild a Go runtime, but that's too hard. 
