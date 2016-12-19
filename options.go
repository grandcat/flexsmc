package main

import (
	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/flexsmc/smc"
	"github.com/grandcat/srpc/client"
	"github.com/grandcat/srpc/server"
)

type Options struct {
	// Certificate for TLS Client Auth and Identification.
	CertFile string
	KeyFile  string
	// Interface specifies the interface to pin for discovery.
	Inteface string
	// Pairing and registration.
	NodeInfo   string
	UsePairing bool
}

type GWOptions struct {
	Registry        *directory.Registry
	SRpcOpts        []server.Option
	AnnounceService bool
	Options
}

type PeerOptions struct {
	GatewayID  string
	smcBackend smc.Connector
	SRpcOpts   []client.Option
	Options
}
