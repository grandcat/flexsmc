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

	ctxMods, cancelMods := context.WithTimeout(context.Background(), time.Second*120)
	defer cancelMods()
	var wg sync.WaitGroup

	// Start regular health report
	healthReporter := healthReporter{
		ctx:          ctxMods,
		wg:           &wg,
		gwConn:       c,
		pingInterval: time.Second * 10,
	}
	healthReporter.start()

	// XXX: wait some time until first ping arrived to register our node to the
	//      internal DB.
	time.Sleep(time.Millisecond * 500)

	// Test2: receive stream of SMCCmds
	advisor := smcAdvisor{
		ctx:     ctxMods,
		wg:      &wg,
		gwConn:  c,
		smcConn: p.smcConn,
	}
	// SpawnListener ends at the moment when finishing a SMC session.
	// Need to respawn on time.
	advisor.ContinousSpawn()

	wg.Wait()
	cancelMods()
	log.Println(">>Canceled mods two times")
	// Reaching this point means either
	// * the connection is lost (e.g. GW is done, different network)
	// *

	p.srpc.TearDown()
}

type healthReporter struct {
	ctx context.Context
	wg  *sync.WaitGroup //< notify master routine when done due to error or ctx

	gwConn proto.GatewayClient

	pingInterval time.Duration
}

func (h *healthReporter) start() {
	h.wg.Add(1)
	go h.report()
}

func (h *healthReporter) report() {
	defer h.wg.Done()

	ticker := time.NewTicker(h.pingInterval)
	defer ticker.Stop()

	for {
		// Ping
		resp, err := h.gwConn.Ping(h.ctx, &proto.SMCInfo{12345})
		if err != nil {
			log.Printf("Could not ping GW: %v", err)
			return
		}
		log.Println("Ping resp:", resp.Status)

		select {
		case <-ticker.C:
			// Do nothing and ping again on next round :-)

		case <-h.ctx.Done():
			// Abort by context
			log.Printf("Health reporter aborted: %v", h.ctx.Err())
			return
		}
	}
}

// smcAdvisor receives SMC jobs, schedules them to available resources of the SMC
// backend and manages the low-level connection handling between GW and this peer.
type smcAdvisor struct {
	ctx context.Context
	// Waitgroup notifies the main routine if all of our routines are done
	wg *sync.WaitGroup

	gwConn  proto.GatewayClient
	smcConn smc.Connector
}

func (s *smcAdvisor) ContinousSpawn() {
	for {
		if err := s.SpawnListener(); err != nil {
			break
		}
		time.Sleep(time.Second)
	}

}

func (s *smcAdvisor) SpawnListener() error {
	// Blocks until stand-by session from SMC backend is available for Reservation.
	smcSession, err := s.smcConn.RequestSession(s.ctx)
	if err != nil {
		log.Printf("Reservation of SMC backend failed: %v", err)
		return err
	}
	// once a stand-by SMC instance is available, a C&C channel is established
	// to receive SMC commands.
	stream, err := s.gwConn.AwaitSMCRound(s.ctx)
	if err != nil {
		smcSession.TearDown()
		log.Printf("Could not receive SMC cmds: %v", err)
		return err
	}

	s.wg.Add(1)
	go s.smc(stream, smcSession)
	log.Printf("Spawned new listener routine for SMC channel to GW")

	return nil
}

func (s *smcAdvisor) smc(stream proto.Gateway_AwaitSMCRoundClient, smcSess smc.Session) {
	defer s.wg.Done()
	defer smcSess.TearDown()

	moreCmds := true
	var cntCmds uint

	for moreCmds {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("%v.ListFeatures(_) = _, %v", s.gwConn, err)
			break
		}
		log.Printf(">> [%v] SMC Cmd: %v", time.Now(), in)
		// Initialize session on first interaction.
		if cntCmds == 0 {
			smcSess.Init(stream.Context(), in.SessionID)
		}
		// Trigger state machine in SMC backend and send generated response.
		var resp *proto.CmdResult
		resp, moreCmds = smcSess.NextCmd(in)
		// Send back response
		log.Printf(">>[%v] Reply to GW now", moreCmds)
		stream.Send(resp)

		cntCmds++
	}
	// A new stream is created for the next SMC round. So close this one.
	// XXX: reuse stream to save resources, but requires stream management
	stream.CloseSend()

}
