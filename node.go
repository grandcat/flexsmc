package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	gtypeAny "github.com/golang/protobuf/ptypes/any"
	"github.com/grandcat/flexsmc/directory"
	proto "github.com/grandcat/flexsmc/proto"
	auth "github.com/grandcat/srpc/authentication"
	"github.com/grandcat/srpc/client"
	"github.com/grandcat/srpc/pairing"
	"github.com/grandcat/srpc/server"
	"golang.org/x/net/context"
)

var (
	isGateway = flag.Bool("gateway", false, "Set to true to run this node as a gateway")
	certFile  = flag.String("cert_file", "certs/cert_server.pem", "TLS cert file")
	keyFile   = flag.String("key_file", "certs/key_server.pem", "TLS key file")
	enPairing = flag.Bool("enPairing", true, "Enable or disable pairing phase")
	peerInfo  = flag.String("peerinfo", "123", "Additional peer information supplied during pairing")
)

var (
	ErrPermDenied = errors.New("access denied")
)

type Node struct {
	// sRPC mandatory base layer
	server.Server

	// Own members follow here
	reg *directory.Registry
}

func (n *Node) Ping(ctx context.Context, in *proto.SMCInfo) (*proto.CmdResult, error) {
	a, ok := auth.FromAuthContext(ctx)
	if ok {
		p := n.reg.GetOrCreate(a.ID)
		log.Println("Last peer ", a.ID, "IP:", a.Addr, " activity:", p.LastActivity())
		p.Touch(a.Addr)

		return &proto.CmdResult{Status: proto.CmdResult_SUCCESS}, nil
	}
	return nil, ErrPermDenied
}

func (n *Node) AwaitSMCCommands(in *proto.SMCInfo, stream proto.Gateway_AwaitSMCCommandsServer) error {
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

	// Await commands for our requesting peer and send it to him.
	// If there is no communication with this specific peer for a while,
	// we need to check whether it is still alive. Otherwise, this peer is
	// shutdown and needs to register again for commands.
	rx := p.SubscribeCmdChan()
	if err != nil {
		return err
	}
	defer p.UnsubscribeCmdChan()

	t := time.NewTicker(time.Second * 30)
	defer t.Stop()
	for {
		select {
		// Incoming commands for TX
		case m, ok := <-rx:
			if !ok {
				return fmt.Errorf("AwaitSMCCommands: chan closed externally")
			}
			cmd := m.(proto.SMCCmd)
			log.Printf("M -> %s: %v", a.ID, m)
			if err := stream.Send(&cmd); err != nil {
				return err
			}
		// Periodic activity check
		// We cannot fully rely on gRPC send a notification via the context. Especially
		// for long lasting streams with few communication, it is recommend to
		// keep the connection alive or tear it down.
		case <-t.C:
			act := p.LastActivity().Seconds()
			if act > directory.MaxActivityGap {
				// Shutdown instruction channel as the peer is probably offline.
				return fmt.Errorf("no activity for too long")
			}
		// Handling remote peer gracefully shutting down stream connection
		case <-streamCtx.Done():
			log.Printf("Conn to peer %s died unexpectedly", a.ID)
			// Stopping ticker and unsubscribing from chan is done here (-> defer)
			return nil
		}
	}

	return nil
}

func runGateway() {
	log.Println("Running as gateway...")
	// Configure server with pairing module
	tlsKeyPrim := server.TLSKeyFile(*certFile, *keyFile)
	n := &Node{server.NewServer(tlsKeyPrim), directory.NewRegistry()}
	mPairing := pairing.NewServerApproval(n.GetPeerCerts(), gtypeAny.Any{"flexsmc/peerinfo", []byte(*peerInfo)})
	n.RegisterModules(mPairing)

	g, err := n.Build()
	if err != nil {
		panic("Server build err:" + err.Error())
	}

	// Register Node RPCs
	proto.RegisterGatewayServer(g, n)
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

	// Start serving (blocks...)
	n.Serve()
}

// runPeer starts a regular SMC peer node. It looks for an interesting gateway with matching properties,
// starts pairing phase if it is an unknown identity and joins the SMC network as a slave. Still,
// it holds the same data as the gateway to be a potential failover candidate if the gateway breaks.
func runPeer() {
	log.Println("Running as regular peer...")
	// 1. Disover potential gateway
	// XXX: assume fixed one for now
	const peerID = "gw4242.flexsmc.local"
	// Configure client with TLS client auth and a pairing module for an unknown gateway
	tlsKeyPrim := client.TLSKeyFile(*certFile, *keyFile)
	n := client.NewClient(tlsKeyPrim)
	// XXX: replace with restart
	// defer n.TearDown()

	if err := n.GetPeerCerts().LoadFromPath("peer1/"); err != nil {
		fmt.Println(err)
	}

	// 2. Initiate pairing if it is an unknown identity (if desired)
	// knownGW := n.GetPeerCerts().ActivePeerCertificates(peerID)
	knownGW := 0
	log.Println("Pairing active?", *enPairing)
	if *enPairing && knownGW == 0 {
		log.Println("Start pairing...")
		// XXX: assume we want to pair with this gateway (e.g. matching properties, instructed by admin, ...)
		ccp, err := n.DialUnsecure(peerID)
		if err != nil {
			log.Printf("Could not initiate pairing to GW node %s: %v", peerID, err)
			return
		}
		log.Println("DialUnsecured done.")
		mPairing := pairing.NewClientApproval(n.GetPeerCerts(), ccp)
		ctx, _ := context.WithTimeout(context.Background(), time.Second*10)

		gwIdentity, err := mPairing.StartPairing(ctx, &gtypeAny.Any{"flexsmc/peerinfo", []byte(*peerInfo)})
		if err != nil {
			log.Printf("Pairing with %s failed: %v", peerID, err)
			return
		}
		// Pairing commissioning
		log.Println("GWIdentity:", gwIdentity.Fingerprint(), "Info:", gwIdentity.Details())
		// XXX: accept GW without out-of-band verification for now
		gwIdentity.Accept()
		n.GetPeerCerts().StoreToPath("peer1/")
		// Wait for server to accept our pairing request
		status := mPairing.AwaitPairingResult(ctx)
		if r, ok := <-status; ok {
			log.Println("Pairing: peer responded with", r)
		} else {
			log.Println("Pairing aborted by peer")
		}
	}

	// Join the SMC network.
	cc, err := n.Dial(peerID)
	if err != nil {
		log.Printf("Could not connect to GW node %s: %v", peerID, err)
		return
	}
	c := proto.NewGatewayClient(cc)

	// Test1: ping our gateway
	go func() {
		for i := 0; i < 4; i++ {
			ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
			resp, err := c.Ping(ctx, &proto.SMCInfo{12345})
			if err != nil {
				log.Printf("Could not ping GW %s: %v", peerID, err)
				return
			}
			log.Println("Ping resp:", resp.Status)

			time.Sleep(time.Second * 10)
		}
	}()
	// XXX: wait some time until first ping arrived to register our node to the
	//      internal DB.
	time.Sleep(time.Millisecond * 250)

	// Test2: receive stream of SMCCmds
	myInfo := &proto.SMCInfo{Tcpport: 42424}
	stream, err := c.AwaitSMCCommands(context.Background(), myInfo)
	if err != nil {
		log.Printf("Could not receive GW's SMC cmds: %v", err)
		return
	}
	for {
		m, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("%v.ListFeatures(_) = _, %v", c, err)
		}
		log.Printf(">> [%v] SMC Cmd: %v", time.Now(), m)
		switch cmd := m.Payload.(type) {
		case *proto.SMCCmd_Prepare:
			log.Println(">> Participants:", cmd.Prepare.Participants)
		}
	}

	n.TearDown()

}

func main() {
	flag.Parse()

	if *isGateway {
		// Gateway role
		runGateway()
	} else {
		// Normal node role
		runPeer()
	}
}
