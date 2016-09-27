package main

import "flag"

var (
	isGateway = flag.Bool("gateway", false, "Set to true to run this node as a gateway")
	certFile  = flag.String("cert_file", "certs/cert_server.pem", "TLS cert file")
	keyFile   = flag.String("key_file", "certs/key_server.pem", "TLS key file")
	enPairing = flag.Bool("enPairing", true, "Enable or disable pairing phase")
	peerInfo  = flag.String("peerinfo", "123", "Additional peer information supplied during pairing")
)

func runGateway() {
	opts := GWOptions{
		Options: Options{
			CertFile: *certFile,
			KeyFile:  *keyFile,
			NodeInfo: *peerInfo,
		},
	}
	gw := NewGateway(opts)
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
