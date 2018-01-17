package main

import (
	"context"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	bc "github.com/relab/bchain/bchain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type peer struct {
	sync.Mutex
	haveSucc bool

	addr          string
	keyFile       string
	chainMsgQueue chan *bc.ChainMsg
	ackMsgQueue   chan *bc.AckMsg
}

// NewPeer creates a new peer running on the provided adr.
func NewPeer(adr, keyFile string) *peer {
	return &peer{
		addr:          adr,
		keyFile:       keyFile,
		chainMsgQueue: make(chan *bc.ChainMsg, 10),
		ackMsgQueue:   make(chan *bc.AckMsg, 10),
	}
}

func (p *peer) Serve() {
	// this check is to avoid firewall dialog on macOS, when just passing ":port"
	if strings.HasPrefix(p.addr, ":") {
		p.addr = "localhost" + p.addr
	}
	l, err := net.Listen("tcp", p.addr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	opts := serverOptions(p.keyFile)
	grpcServer := grpc.NewServer(opts...)
	bc.RegisterBChainServer(grpcServer, p)
	log.Printf("server %s running", l.Addr())
	log.Fatal(grpcServer.Serve(l))
}

func serverOptions(keyFile string) []grpc.ServerOption {
	opts := []grpc.ServerOption{}
	if keyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(keyFile+".crt", keyFile+".key")
		if err != nil {
			log.Fatalf("failed to load credentials: %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	return opts
}

func (p *peer) connect(dialCtx context.Context, server string) {
	conn, err := grpc.DialContext(dialCtx, server, dialOptions(p.keyFile), grpc.WithBlock())
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := bc.NewBChainClient(conn)
	md := metadata.MD{
		"key": []string{"val"},
	}
	ctx := metadata.NewOutgoingContext(dialCtx, md)
	stream, err := client.Chain(ctx)
	if err != nil {
		log.Fatalf("client.Chain(_) = _, %v\n", err)
	}
	waitc := make(chan struct{})
	go func() {
		for {
			ack, err := stream.Recv()
			if err == io.EOF {
				close(waitc)
				return
			}
			if err != nil {
				log.Fatalf("failed to receive ack: %v", err)
			}
			log.Printf("Ack for ID: %d", ack.ID)
			p.ackMsgQueue <- ack
		}
	}()
	p.Lock()
	p.haveSucc = true
	p.Unlock()

	//TODO CHECK IF THIS WORKS
	// for {
	// 	select {
	// 	case chainMsg := <-p.chainMsgQueue:
	// 		if err := stream.Send(chainMsg); err != nil {
	// 			log.Fatalf("failed to send chain msg: %v", err)
	// 		}
	// 	case <-dialCtx.Done():// OR some other way to close the connection
	// 		stream.CloseSend()
	// 		return
	// 	}
	// }
	for chainMsg := range p.chainMsgQueue {
		if err := stream.Send(chainMsg); err != nil {
			log.Fatalf("failed to send chain msg: %v", err)
		}
	}
	stream.CloseSend()
	<-waitc
}

func dialOptions(certFile string) grpc.DialOption {
	if certFile == "" {
		return grpc.WithInsecure()
	}
	clientCreds, err := credentials.NewClientTLSFromFile(certFile+".crt", "127.0.0.1")
	if err != nil {
		log.Fatalf("error creating credentials: %v", err)
	}
	return grpc.WithTransportCredentials(clientCreds)
}

func (p *peer) haveSuccessor() bool {
	//TODO We can avoid this method by blocking on Chain() waiting for sync.Once() to determine if haveSucc
	p.Lock()
	defer p.Unlock()
	return p.haveSucc
}

// Chain is the RPC server method called by BChain clients to set up a connection
// for receiving ChainMsgs.
//
// This function can technically be called by any of the replicas,
// but only the successor replica's connection will be accepted.
// TODO How can we ensure this? Even when reconfiguring/rechaining.
func (p *peer) Chain(stream bc.BChain_ChainServer) error {
	md, ok := metadata.FromIncomingContext(stream.Context())
	if ok {
		if err := stream.SendHeader(md); err != nil {
			return grpc.Errorf(grpc.Code(err), "%v.SendHeader(%v) = %v, want %v", stream, md, err, nil)
		}
		// stream.SetTrailer(testTrailerMetadata)
	}

	for {
		chainMsg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		// if tail:= p.chain.SendSucc(chainMsg); tail { ack }
		if p.haveSuccessor() {
			log.Printf("[%s]: enqueing %v", p.addr, chainMsg)
			p.chainMsgQueue <- chainMsg
		} else {
			log.Printf("[%s]: i'm the tail %v", p.addr, chainMsg)
			p.ackMsgQueue <- &bc.AckMsg{ID: chainMsg.ID}
		}

		go func() {
			for ackMsg := range p.ackMsgQueue {
				if err := stream.Send(ackMsg); err != nil {
					log.Fatalf("failed to send ack msg: %v", err)
				}
			}
		}()
	}
}
