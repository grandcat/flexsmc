package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	gtypeAny "github.com/golang/protobuf/ptypes/any"
	"github.com/grandcat/flexsmc/modules"
	proto "github.com/grandcat/flexsmc/proto"
	"github.com/grandcat/flexsmc/smc"
	"github.com/grandcat/srpc/client"
	"github.com/grandcat/srpc/pairing"
	"golang.org/x/net/context"
)

type RunState uint8

const (
	Discovery RunState = iota
	Connecting
	Connected
	Running
)

type Peer struct {
	srpc    *client.Client
	smcConn smc.Connector
	opts    PeerOptions
}

func NewPeer(opts PeerOptions) *Peer {
	srpcOpts := append(opts.SRpcOpts, client.TLSKeyFile(*certFile, *keyFile))

	smcConn := opts.smcBackend
	if smcConn == nil {
		smcConn = smc.DefaultSMCConnector
	}

	return &Peer{
		srpc:    client.NewClient(srpcOpts...),
		smcConn: smcConn,
		opts:    opts,
	}
}

func (p *Peer) Init() {
	if err := p.srpc.GetPeerCerts().LoadFromPath("peer1/"); err != nil {
		fmt.Println(err)
	}
}

func (p *Peer) StateMachine() {

}

func (p *Peer) Run() {
	log.Println("Starting peer operation...")
	// 1. Disover potential gateway
	// XXX: assume fixed one for now
	const peerID = "gw4242.flexsmc.local"

	// 2. Initiate pairing if it is an unknown identity (if desired)
	// knownGW := p.srpc.GetPeerCerts().ActivePeerCertificates(peerID)
	knownGW := 0
	log.Println("Pairing active?", p.opts.UsePairing)
	if p.opts.UsePairing && knownGW == 0 {
		log.Println("Start pairing...")
		// XXX: assume we want to pair with this gateway (e.g. matching properties, instructed by admin, ...)
		ccp, err := p.srpc.DialUnsecure(peerID)
		if err != nil {
			log.Printf("Could not initiate pairing to GW node %s: %v", peerID, err)
			return
		}
		log.Println("DialUnsecured done.")
		mPairing := pairing.NewClientApproval(p.srpc.GetPeerCerts(), ccp)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		gwIdentity, err := mPairing.StartPairing(ctx, &gtypeAny.Any{"flexsmc/peerinfo", []byte(p.opts.NodeInfo)})
		if err != nil {
			log.Printf("Pairing with %s failed: %v", peerID, err)
			return
		}
		// Pairing commissioning
		log.Println("GWIdentity:", gwIdentity.Fingerprint(), "Info:", gwIdentity.Details())
		// XXX: accept GW without out-of-band verification for now
		gwIdentity.Accept()
		p.srpc.GetPeerCerts().StoreToPath("peer1/")
		// Wait for server to accept our pairing request
		status := mPairing.AwaitPairingResult(ctx)
		if r, ok := <-status; ok {
			log.Println("Pairing: peer responded with", r)
		} else {
			log.Println("Pairing aborted by peer")
		}
	}

	// Join the SMC network.
	cc, err := p.srpc.Dial(peerID)
	if err != nil {
		log.Printf("Could not connect to GW node %s: %v", peerID, err)
		return
	}
	c := proto.NewGatewayClient(cc)

	ctxMods, cancelMods := context.WithTimeout(context.Background(), time.Second*120)
	defer cancelMods()
	modInfo := modules.ModuleContext{
		Context:    ctxMods,
		ActiveMods: &sync.WaitGroup{},
		GWConn:     c,
	}

	// Start regular health report
	healthMod := modules.NewHealthReporter(modInfo, time.Second*10)
	healthMod.Start()

	time.Sleep(time.Millisecond * 500)

	// Test2: receive stream of SMCCmds
	smcAdvisor := modules.NewSMCAdvisor(modInfo, p.smcConn)
	// SpawnListener ends at the moment when finishing a SMC session.
	// Need to respawn on time.
	smcAdvisor.BlockingSpawn()

	modInfo.ActiveMods.Wait()
	// Reaching this point means either
	// * the connection is lost (e.g. GW is done, different network)
	// * context is done
	cancelMods()
	log.Println(">>Canceled mods two times")

	p.srpc.TearDown()
}
