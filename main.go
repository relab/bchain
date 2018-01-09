package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	bc "github.com/relab/bchain/bchain"
)

//go:generate protoc -I=$GOPATH/src/:. --go_out=plugins=grpc:. bchain/bchain.proto

func main() {
	var (
		myidx      = flag.Int("idx", -1, "the index of the local server (only needed when running on localhost)")
		saddrs     = flag.String("addrs", "", "server addresses separated by ','")
		keyFile    = flag.String("key", "keys/server", "name of public/private key file and certificate this server")
		cpuprofile = flag.String("cpuprofile", "", "write cpu profile `file`")
		memprofile = flag.String("memprofile", "", "write memory profile to `file`")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func() {
			<-c
			pprof.StopCPUProfile()
			os.Exit(1)
		}()
	}

	addrs, myAddr := getAddrs(*saddrs, *myidx)

	// run peer server in background
	peer := NewPeer(myAddr, *keyFile)
	go peer.Serve()
	// set up and connect to chain
	chain := NewChain(addrs, peer)
	// connect to chain (this peer's successor)
	// wait for one minute to connect if server isn't up yet
	if isHead := chain.Connect(time.Minute); isHead {
		log.Println("I'm the head of the chain!")
		for i := 0; i < 10; i++ {
			chainMsg := &bc.ChainMsg{
				ID: int64(i),
				Op: fmt.Sprintf("SaveOp from %v", myAddr),
			}
			log.Printf("[%s]: enqueing %v", peer.addr, chainMsg)
			peer.chainMsgQueue <- chainMsg
			time.Sleep(10 * time.Millisecond)
		}
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
		f.Close()
	}
	select {}
}

// getAddrs returns the hosts to connect to and the local address (host-port pair).
// If myidx is -1, then we try to find the hostname in the provided list.
// Otherwise, we use myidx as an index into the provided list (starting at 0).
func getAddrs(saddrs string, myidx int) ([]string, string) {
	addrs := strings.Split(saddrs, ",")
	if len(addrs) == 1 && addrs[0] == saddrs {
		log.Fatal("no server addresses provided")
	}
	if len(addrs) < 4 {
		log.Fatalf("need at least 3f+1=4 addresses, only got %d", len(addrs))
	}
	if myidx == -1 {
		// Search for hostname in provide list of addresses
		myHost, err := os.Hostname()
		log.Printf("hostname: %v", myHost)
		if err != nil {
			log.Fatal(err)
		}
		for _, adr := range addrs {
			log.Printf("adr: %v", adr)
			host, _, err := net.SplitHostPort(adr)
			if err != nil {
				log.Fatal(err)
			}
			if host == myHost {
				return addrs, adr
			}
		}
		log.Fatalf("couldn't find '%s' in provided addrs (%v)", myHost, addrs)
	}
	if len(addrs) < myidx {
		log.Fatalf("index out of bounds %d; must be less than %d", myidx, len(addrs))
	}
	log.Printf("#addrs: %d (%v)", len(addrs), addrs)
	return addrs, addrs[myidx]
}
