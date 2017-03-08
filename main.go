package main

import (
	"context"
	"flag"
	"time"

	"github.com/golang/glog"
	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/flexsmc/node"
	"github.com/grandcat/flexsmc/orchestration"
	pbJob "github.com/grandcat/flexsmc/proto/job"
	"github.com/grandcat/flexsmc/smc"
)

var (
	isGateway = flag.Bool("gateway", false, "Set to true to run this node as a gateway")
	certFile  = flag.String("cert_file", "certs/cert_01.pem", "TLS cert file")
	keyFile   = flag.String("key_file", "certs/key_01.pem", "TLS key file")
	iface     = flag.String("interface", "", "Network interface to use for discovery, e.g. enp3s0")
	gwID      = flag.String("gw_id", "n01.flexsmc.local", "Gateway mDNS identifer (will be obsolete in future)")
	enPairing = flag.Bool("pairing", false, "Enable or disable pairing phase")
	peerInfo  = flag.String("peerinfo", "123", "Additional peer information supplied during pairing")

	smcProvider = flag.String("smcsocket", "", "Custom bind address for SMC provider to pass to SMC connector")
)

func runGateway() {
	registry := directory.NewRegistry()

	opts := node.GWOptions{
		Options: node.Options{
			CertFile:   *certFile,
			KeyFile:    *keyFile,
			Inteface:   *iface,
			NodeInfo:   *peerInfo,
			UsePairing: *enPairing,
		},
		Registry:        registry,
		AnnounceService: true,
	}
	gw := node.NewGateway(opts)
	// Invoke some fake client requests
	orchestration := orchestration.NewFIFOOrchestration(registry)
	go func() {
		for {
			time.Sleep(time.Second * 10)
			glog.V(1).Infoln("Submit SMC task to worker pool")

			jobTimeout, cancel := context.WithTimeout(context.Background(), time.Second*20)
			res, err := orchestration.Request(jobTimeout, &pbJob.SMCTask{Set: "dummygroup", Aggregator: pbJob.Aggregator_SUM})
			glog.V(1).Infof("END RES:\n>>>>>>>>> %v [Error: %v] <<<<<<<<<<<", res, err)

			// XXX: prevent memory leak, so release resources when done.
			cancel()
		}
	}()

	// Start (blocking) GW operation
	gw.Run()
}

// runPeer starts a regular SMC peer node. It looks for an interesting gateway with matching properties,
// starts pairing phase if it is an unknown identity and joins the SMC network as a slave. Still,
// it holds the same data as the gateway to be a potential failover candidate if the gateway breaks.
func runPeer() {
	opts := node.PeerOptions{
		Options: node.Options{
			CertFile:   *certFile,
			KeyFile:    *keyFile,
			NodeInfo:   *peerInfo,
			UsePairing: *enPairing,
		},
		GatewayID:  *gwID,
		SmcBackend: smc.DefaultSMCConnector(*smcProvider),
	}
	peer := node.NewPeer(opts)
	peer.Init()
	peer.Run()
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
