package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	gtypeAny "github.com/golang/protobuf/ptypes/any"
	gtypeEmpty "github.com/golang/protobuf/ptypes/empty"
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
}

func (n *Node) Ping(ctx context.Context, in *gtypeEmpty.Empty) (*proto.CmdResult, error) {
	a, ok := auth.FromAuthContext(ctx)
	if ok {
		log.Println(">>PingCTX:", a.PeerID)
		// TODO: verify peer identity in directory

		return &proto.CmdResult{Status: proto.CmdResult_SUCCESS}, nil
	}
	return nil, ErrPermDenied
}

func runGateway() {
	log.Println("Running as gateway...")
	// Configure server with pairing module
	tlsKeyPrim := server.TLSKeyFile(*certFile, *keyFile)
	n := &Node{server.NewServer(tlsKeyPrim)}
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

	for i := 0; i < 3; i++ {
		ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
		resp, err := c.Ping(ctx, &gtypeEmpty.Empty{})
		if err != nil {
			log.Printf("Could not ping GW %s: %v", peerID, err)
			return
		}
		log.Println("Ping resp:", resp.Status)
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
