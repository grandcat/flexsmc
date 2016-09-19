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
	// sRPC base layer
	server.Server

	// Own members follow here
	reg *directory.Registry
}

func (n *Node) Ping(ctx context.Context, in *proto.SMCInfo) (*proto.CmdResult, error) {
	a, ok := auth.FromAuthContext(ctx)
	if ok {
		log.Println("Last peer ", a.ID, "IP:", a.Addr, " activity:", n.reg.LastActivity(a.ID))
		n.reg.Touch(a.ID, a.Addr, uint16(in.Tcpport))

		return &proto.CmdResult{Status: proto.CmdResult_SUCCESS}, nil
	}
	return nil, ErrPermDenied
}

func (n *Node) AwaitSMCCommands(in *proto.SMCInfo, stream proto.Gateway_AwaitSMCCommandsServer) error {
	log.Println(">>Stream context:", stream.Context())

	for i := 0; i < 3; i++ {
		if i == 1 {
			time.Sleep(time.Second)
		}
		if err := stream.Send(&proto.SMCCmd{Type: proto.SMCCmd_Type(i)}); err != nil {
			return err
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
	for i := 0; i < 3; i++ {
		ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
		resp, err := c.Ping(ctx, &proto.SMCInfo{12345})
		if err != nil {
			log.Printf("Could not ping GW %s: %v", peerID, err)
			return
		}
		log.Println("Ping resp:", resp.Status)
	}
	// Test2: receive stream of SMCCmds
	myInfo := &proto.SMCInfo{Tcpport: 42424}
	stream, err := c.AwaitSMCCommands(context.Background(), myInfo)
	if err != nil {
		log.Printf("Could not receive GW's SMC cmds: %v", err)
		return
	}
	for {
		cmd, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("%v.ListFeatures(_) = _, %v", c, err)
		}
		log.Printf(">> [%v] SMC Cmd: %v", time.Now(), cmd)
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
