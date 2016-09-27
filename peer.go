package main

import (
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	gtypeAny "github.com/golang/protobuf/ptypes/any"
	proto "github.com/grandcat/flexsmc/proto"
	"github.com/grandcat/flexsmc/smc"
	"github.com/grandcat/srpc/client"
	"github.com/grandcat/srpc/pairing"
	"golang.org/x/net/context"
)

type Peer struct {
	srpc *client.Client
	opts PeerOptions
}

func NewPeer(opts PeerOptions) *Peer {
	srpcOpts := append(opts.SRpcOpts, client.TLSKeyFile(*certFile, *keyFile))

	return &Peer{
		srpc: client.NewClient(srpcOpts...),
		opts: opts,
	}
}

func (p *Peer) Init() {
	if err := p.srpc.GetPeerCerts().LoadFromPath("peer1/"); err != nil {
		fmt.Println(err)
	}
}

func (p *Peer) RunStateMachine() {

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
	var wg sync.WaitGroup
	wg.Add(1)

	// Test1: ping our gateway
	go func() {
		defer wg.Done()
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

	// // Test2: receive stream of SMCCmds
	stream, err := c.AwaitSMCRound(context.Background())
	if err != nil {
		log.Printf("Could not receive GW's SMC cmds: %v", err)
		return
	}

	cmdNum := 0
	smcConn := smc.DefaultSMCSession()
	for {
		m, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("%v.ListFeatures(_) = _, %v", c, err)
		}
		log.Printf(">> [%v] SMC Cmd: %v", time.Now(), m)
		var resp *proto.CmdResult
		switch cmd := m.Payload.(type) {
		case *proto.SMCCmd_Prepare:
			log.Println(">> Participants:", cmd.Prepare.Participants)
			resp = smcConn.Prepare(cmd.Prepare)
		case *proto.SMCCmd_Session:
			log.Println(">> Session phase:", cmd.Session)
			resp = <-smcConn.DoSession(cmd.Session)
		}
		// XXX: need a lot of time for session phase ;)
		// if *certFile == "certs/cert_client3.pem" && cmdNum == 1 {
		// 	time.Sleep(time.Second * 10)
		// }
		// Send back response
		log.Println(">> Send msg to GW now")
		stream.Send(resp)

		cmdNum++
	}

	p.srpc.TearDown()
	wg.Wait()
}

// gwInteraction interacts with the GW while it is connected to.
type gwInteraction struct {
	c proto.GatewayClient
}

// smcAdvisor redirects SMC jobs to a SMC backend and sends back the result.
type smcAdvisor struct {
	c proto.GatewayClient
}

// func (sa *smcAdvisor) Start() {
// 	sa.c.AwaitSMCRound()
// }
