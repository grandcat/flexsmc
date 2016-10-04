package main

import (
	"errors"
	"fmt"
	"log"
	"time"

	gtypeAny "github.com/golang/protobuf/ptypes/any"
	"github.com/grandcat/flexsmc/directory"
	proto "github.com/grandcat/flexsmc/proto"
	"github.com/grandcat/flexsmc/wiring"
	auth "github.com/grandcat/srpc/authentication"
	"github.com/grandcat/srpc/pairing"
	"github.com/grandcat/srpc/server"
	"golang.org/x/net/context"
)

var (
	ErrPermDenied = errors.New("access denied")
)

type Gateway struct {
	// sRPC mandatory base layer
	server.Server
	// Own members follow here
	reg  *directory.Registry
	opts GWOptions
}

func NewGateway(opts GWOptions) *Gateway {
	// srpc configuration
	srpcOpts := append(opts.SRpcOpts, server.TLSKeyFile(opts.CertFile, opts.KeyFile))
	srpc := server.NewServer(srpcOpts...)
	// GW configuration
	reg := opts.Registry
	if reg == nil {
		reg = directory.NewRegistry()
	}

	return &Gateway{
		Server: srpc,
		reg:    reg,
		opts:   opts,
	}
}

// RPC service

func (n *Gateway) Ping(ctx context.Context, in *proto.SMCInfo) (*proto.CmdResult, error) {
	a, ok := auth.FromAuthContext(ctx)
	if ok {
		p := n.reg.GetOrCreate(a.ID)
		log.Println("Last peer ", a.ID, "IP:", a.Addr, " activity:", p.LastActivity())
		p.Touch(a.Addr)

		return &proto.CmdResult{Status: proto.CmdResult_SUCCESS}, nil
	}
	return nil, ErrPermDenied
}

func (n *Gateway) AwaitSMCRound(stream proto.Gateway_AwaitSMCRoundServer) error {
	streamCtx := stream.Context()
	a, _ := auth.FromAuthContext(streamCtx)
	log.Println(">>Stream requested from:", a.ID)
	// Implicitly notify the registry of peer activity so the first call
	// to ping() RPC can come later.
	p, err := n.reg.Get(a.ID)
	if err != nil {
		return err
	}
	p.Touch(a.Addr)

	// Await a C&C channel from the gateway. This means some work is waiting
	// for this peer.
	// In case of no communication with this specific peer for a while,
	// we need to check whether it is still alive. Otherwise, this peer is
	// teared down and needs to register again prior to new jobs.
	gwChat := p.SubscribeCmdChan()
	if err != nil {
		return err
	}
	defer p.UnsubscribeCmdChan()

	t := time.NewTicker(time.Second * 30)
	defer t.Stop()
	for {
		select {
		// GW wants to talk to this peer via the established chat
		case ch := <-gwChat:
			keepAlive, err := chatLoop(stream, ch)
			if err != nil {
				return err
			}
			log.Printf("[%s] chat loop finished.", a.ID)
			// XXX: kill the chat here as we are done. Check if we should keep it and
			// reuse it in favor of recreating a new channel on peer side.
			if !keepAlive {
				return nil
			}

		// Periodic activity check
		// We cannot fully rely on gRPC send a notification via the context. Especially
		// for long lasting streams with few communication, it is recommend to
		// keep the connection alive or tear it down.
		case <-t.C:
			act := p.LastActivity().Seconds()
			if act > directory.MaxActivityGap {
				// Shutdown instruction channel as the peer is probably offline.
				return fmt.Errorf("no ping activity for too long")
			}

		// Handling remote peer gracefully shutting down stream connection
		case <-streamCtx.Done():
			log.Printf("Conn to peer %s teared down unexpectedly", a.ID)
			// Stopping ticker and unsubscribing from chan is done here (-> defer)
			return nil
		}
	}

}

// Ping-ping chat between peer and gateway until one of them tears down the connection.
func chatLoop(stream proto.Gateway_AwaitSMCRoundServer, ch directory.ChatWithGateway) (bool, error) {
	streamCtx := stream.Context()
	a, _ := auth.FromAuthContext(streamCtx)

	fromGW := ch.GetInstructions()
	toGW := ch.Feedback()
	for {
		cmd, more := <-fromGW
		if !more {
			// TODO: notify peer about finished SMC session (or just kill stream?)
			break
		}
		// 1. Send instruction to waiting peer
		log.Printf("GW -> %s: %v", a.ID, cmd)
		if err := stream.Send(cmd); err != nil {
			return false, err
		}
		// 2. Wait for response and forward to waiting GW
		resp, err := stream.Recv()
		if err != nil {
			// Inform GW about loss of connection
			toGW <- &proto.CmdResult{Status: proto.CmdResult_STREAM_ERR}
			log.Printf("[%s] stream rcv aborted.", a.ID)
			return false, err
		}
		ch.SetPeerMetadata(resp)
		toGW <- resp
	}
	// TODO: check if peer wants to keep this stream. Default: close it
	return false, nil
}

// GW operation

func (g *Gateway) Run() {
	log.Println("Starting GW operation...")

	mPairing := pairing.NewServerApproval(g.GetPeerCerts(), gtypeAny.Any{"flexsmc/peerinfo", []byte(g.opts.NodeInfo)})
	g.RegisterModules(mPairing)

	grpc, err := g.Build()
	if err != nil {
		panic("Server build err:" + err.Error())
	}

	// Register RPCs
	proto.RegisterGatewayServer(grpc, g)
	// XXX: Control pairing
	go func() {
		log.Println("GW: waiting for pairings")
		registered := mPairing.IncomingRequests()
		for {
			select {
			case pID := <-registered:
				log.Println("Incoming registration from:", pID.Fingerprint(), "with details:", pID.Details())
				time.Sleep(time.Second * 2) //< Simulate an out-of-band verification. Takes some time...
				pID.Accept()
			}
		}

	}()
	// XXX: send msg to peers
	go func() {
		time.Sleep(time.Second * 10)
		log.Println(">>GW: try sending message to peer")
		comm := wiring.NewPeerConnection(g.reg)
		// n.reg.Watcher.AvailableNodes
		// Declare message for transmission
		for {
			// Send job to online peers
			m := proto.SMCCmd{
				State: proto.SMCCmd_PREPARE,
				Payload: &proto.SMCCmd_Prepare{&proto.Prepare{
					Participants: []*proto.Prepare_Participant{&proto.Prepare_Participant{Addr: "myAddr", Identity: "ident"}},
				}},
			}
			// Submit to online peers
			jobTimeout, cancel := context.WithTimeout(context.Background(), time.Second*8)
			defer cancel()
			job, _ := comm.SubmitJob(jobTimeout, g.reg.Watcher.AvailablePeers(), &m)

			for {
				res, ok := <-job.Result()
				if !ok {
					log.Println(">> GW: feedback channel closed:", job.Err())
					break
				}
				log.Println(">> GW: RESULT FROM PEER:", res)
			}
			time.Sleep(time.Second * 7)
		}

	}()

	// Start serving (blocking)
	g.Serve()
}
