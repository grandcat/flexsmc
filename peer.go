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
	// wg.Add(1)

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

	// Test2: receive stream of SMCCmds
	advisor := smcAdvisor{
		ctx:     context.Background(),
		gwConn:  c,
		smcConn: p.smcConn,
		wg:      &wg,
	}
	// SpawnListener ends at the moment when finishing a SMC session.
	// Need to respawn on time.
	advisor.SpawnListener()

	wg.Wait()
	p.srpc.TearDown()
}

// smcAdvisor redirects SMC jobs to a SMC backend and sends back the result.
type smcAdvisor struct {
	ctx     context.Context
	gwConn  proto.GatewayClient
	smcConn smc.Connector
	wg      *sync.WaitGroup
}

func (s *smcAdvisor) SpawnListener() error {
	stream, err := s.gwConn.AwaitSMCRound(s.ctx)
	if err != nil {
		log.Printf("Could not receive SMC cmds: %v", err)
		return err
	}

	s.wg.Add(1)
	go s.smc(stream)
	log.Printf("Spawned new SMC listener routine to GW")

	return nil
}

func (s *smcAdvisor) smc(stream proto.Gateway_AwaitSMCRoundClient) {
	defer s.wg.Done()

	var smcSess smc.Session
	moreCmds := true
	for moreCmds {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("%v.ListFeatures(_) = _, %v", s.gwConn, err)
			break
		}
		log.Printf(">> [%v] SMC Cmd: %v", time.Now(), in)

		// Try allocating SMC session on first command
		if smcSess == nil {
			var smcErr error
			smcSess, smcErr = s.smcConn.Attach(stream.Context(), in.SessionID)
			if smcErr != nil {
				stream.Send(&proto.CmdResult{Status: proto.CmdResult_DENIED, Msg: smcErr.Error()})
				stream.CloseSend()
				break
			}
		}
		// Verify that session did not change
		if in.SessionID != smcSess.ID() {
			stream.Send(&proto.CmdResult{Status: proto.CmdResult_DENIED, Msg: "session change not allowed here"})
			stream.CloseSend()
			break
		}
		// Route cmd to SMC provider
		var resp *proto.CmdResult
		switch cmd := in.Payload.(type) {
		case *proto.SMCCmd_Prepare:
			log.Println(">> Participants:", cmd.Prepare.Participants)
			resp = <-smcSess.Prepare(cmd.Prepare)
		case *proto.SMCCmd_Session:
			log.Println(">> Session phase:", cmd.Session)
			resp = <-smcSess.DoSession(cmd.Session)
			moreCmds = false
		}
		// Send back response
		log.Println(">> Send msg to GW now")
		stream.Send(resp)
	}

}
