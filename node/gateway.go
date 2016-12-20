package node

import (
	"errors"
	"fmt"
	"time"

	"strings"

	"net"

	"github.com/golang/glog"
	gtypeAny "github.com/golang/protobuf/ptypes/any"
	"github.com/grandcat/flexsmc/directory"
	proto "github.com/grandcat/flexsmc/proto"
	pbJob "github.com/grandcat/flexsmc/proto/job"
	auth "github.com/grandcat/srpc/authentication"
	"github.com/grandcat/srpc/pairing"
	"github.com/grandcat/srpc/server"
	"github.com/grandcat/zeroconf"
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
	mDNS *zeroconf.Server
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

func (n *Gateway) Ping(ctx context.Context, in *proto.SMCInfo) (*pbJob.CmdResult, error) {
	a, ok := auth.FromAuthContext(ctx)
	if ok {
		p := n.reg.GetOrCreate(a.ID)
		glog.V(2).Infoln("Last peer ", a.ID, "IP:", a.Addr, " activity:", p.LastActivity())
		p.Touch(a.Addr)

		return &pbJob.CmdResult{Status: pbJob.CmdResult_SUCCESS}, nil
	}
	return nil, ErrPermDenied
}

func (n *Gateway) AwaitSMCRound(stream proto.Gateway_AwaitSMCRoundServer) error {
	streamCtx := stream.Context()
	a, _ := auth.FromAuthContext(streamCtx)

	p := n.reg.GetOrCreate(a.ID)
	p.Touch(a.Addr)

	// Await a C&C channel from the gateway. This means some work is waiting
	// for this peer.
	// In case of no communication with this specific peer for a while,
	// we need to check whether it is still alive. Otherwise, this peer is
	// teared down and needs to register again prior to new jobs.
	gwChat := p.SubscribeCmdChan()
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
			glog.V(2).Infof("[%s] chat loop finished.", a.ID)
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
			glog.V(1).Infof("[%s] conn to peer teared down unexpectedly", a.ID)
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
		glog.V(2).Infof("GW -> [%s]: %v", a.ID, cmd)
		if err := stream.Send(cmd); err != nil {
			return false, err
		}
		// 2. Wait for response and forward to waiting GW
		resp, err := stream.Recv()
		if err != nil {
			// Inform GW about loss of connection
			toGW <- &pbJob.CmdResult{Status: pbJob.CmdResult_STREAM_ERR}
			glog.V(1).Infof("[%s] stream rcv aborted.", a.ID)
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
	glog.Infof("Starting GW: %s", g.Server.CommonName())

	mPairing := pairing.NewServerApproval(g.PeerCerts(), gtypeAny.Any{"flexsmc/peerinfo", []byte(g.opts.NodeInfo)})
	g.RegisterModules(mPairing)

	grpc, err := g.Build()
	if err != nil {
		panic("Server build err:" + err.Error())
	}

	// Register RPCs
	proto.RegisterGatewayServer(grpc, g)
	// XXX: Control pairing
	go func() {
		glog.Infoln("Waiting for pairings")
		registered := mPairing.IncomingRequests()
		for {
			select {
			case pID := <-registered:
				glog.Infoln("Incoming registration from:", pID.Fingerprint(), "with details:", pID.Details())
				time.Sleep(time.Second * 2) //< Simulate an out-of-band verification. Takes some time...
				pID.Accept()
			}
		}
	}()

	if g.opts.AnnounceService {
		g.discovery()
	}
	// Start serving (blocking until Ctrl-c)
	g.Serve()
	g.cleanShutdown()
}

func (g *Gateway) discovery() {
	srv := strings.SplitN(g.Server.CommonName(), ".", 3)
	srvName := srv[0]
	srvType := fmt.Sprintf("_%s._tcp", srv[1])

	// Use custom interface if provided. Otherwise, it is nil and zeroconf will
	// select suitable ones.
	// Note: this is the recommended way until zeroconf lib correctly publishes IPs
	// only on interfaces it supports.
	var ifaces []net.Interface
	if iface, _ := net.InterfaceByName(g.opts.Inteface); iface != nil {
		ifaces = []net.Interface{*iface}
	} else {
		glog.Warningln("Unrecognized interface, using default.")
	}

	var err error
	g.mDNS, err = zeroconf.Register(srvName, srvType, "local", 50051, []string{"txtv=0", "lo=1", "la=2"}, ifaces)
	if err != nil {
		glog.Fatalln("Register mDNS service failed:", err.Error())
	}
}

func (g *Gateway) cleanShutdown() {
	<-g.Server.ShuttingDown()
	glog.Infoln(">> Received shutdown cmd from srpc")

	if g.mDNS != nil {
		g.mDNS.Shutdown()
	}
}
