package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/flexsmc/orchestration"
	proto "github.com/grandcat/flexsmc/proto"
)

var (
	isGateway = flag.Bool("gateway", false, "Set to true to run this node as a gateway")
	certFile  = flag.String("cert_file", "certs/cert_server.pem", "TLS cert file")
	keyFile   = flag.String("key_file", "certs/key_server.pem", "TLS key file")
	enPairing = flag.Bool("enPairing", true, "Enable or disable pairing phase")
	peerInfo  = flag.String("peerinfo", "123", "Additional peer information supplied during pairing")
)

func runGateway() {
	registry := directory.NewRegistry()

	opts := GWOptions{
		Options: Options{
			CertFile: *certFile,
			KeyFile:  *keyFile,
			NodeInfo: *peerInfo,
		},
		Registry: registry,
	}
	gw := NewGateway(opts)
	// Invoke some fake client requests
	orchestration := orchestration.NewFIFOOrchestration(registry)
	go func() {
		for {
			time.Sleep(time.Second * 10)
			log.Println(">>GW: submit SMC task to worker pool")

			jobTimeout, cancel := context.WithTimeout(context.Background(), time.Second*8)
			res, err := orchestration.Request(jobTimeout, &proto.SMCTask{Set: "dummygroup"})
			log.Printf("END RES:\n%v [Error: %v]", res, err)

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
	opts := PeerOptions{
		Options: Options{
			CertFile:   *certFile,
			KeyFile:    *keyFile,
			NodeInfo:   *peerInfo,
			UsePairing: false,
		},
	}
	peer := NewPeer(opts)
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
