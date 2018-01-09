package main

import (
	"context"
	"log"
	"time"
)

// keys for the chain map
const (
	head = iota
	tail
	proxytail
	successor
	predeccessor
)

type Chain struct {
	cmap map[int]string
	peer *peer
}

func NewChain(addrs []string, peer *peer) *Chain {
	c := mapAddrToChain(addrs, peer.addr)
	return &Chain{c, peer}
}

// Connect connects to this peer's successor and
// returns true if this peer is the head of the chain.
func (c *Chain) Connect(wait4ConnDelay time.Duration) bool {
	if succ, ok := c.cmap[successor]; ok {
		log.Println("connecting to:", succ)
		//TODO pass chain or something else in here.
		// set up dial timeout to allow initial connections to wait for the peer to come up
		dialCtx, cancel := context.WithTimeout(context.Background(), wait4ConnDelay)
		//TODO this cancel is problematic?? Need to move context out of Connect()
		defer cancel()
		go c.peer.connect(dialCtx, succ)
	} else {
		log.Println("I'm the tail, so I won't be connecting to anyone!")
	}
	return c.cmap[head] == c.peer.addr
}

func (c *Chain) Close() {
}

//TODO make test cases for this: (using property-based testing)

// mapAddrToChain takes the order of the given addresses and creates a map
// with entries for head, tail, proxy tail, successor, and predeccessor.
func mapAddrToChain(addrs []string, myAddr string) map[int]string {
	log.Printf("addrs: %v (%d)", addrs, len(addrs))
	n := len(addrs)
	if (n-1)%3 != 0 {
		panic("n must be 3f+1")
	}
	f := (n - 1) / 3
	ptail := 2*f + 1
	chain := make(map[int]string)
	chain[head] = addrs[0]
	chain[tail] = addrs[len(addrs)-1]
	chain[proxytail] = addrs[ptail]
	myIdx := 0
	for i, srv := range addrs {
		if srv == myAddr {
			myIdx = i
			break
		}
	}
	log.Printf("myIdx: %d (%s)", myIdx, myAddr)

	if myIdx < len(addrs)-1 {
		chain[successor] = addrs[myIdx+1]
	}
	if myIdx > 0 {
		chain[predeccessor] = addrs[myIdx-1]
	}
	return chain
}
