package main

import (
	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/srpc/client"
	"github.com/grandcat/srpc/server"
)

type Options struct {
	// Certificate for TLS Client Auth and Identification
	CertFile string
	KeyFile  string
	// Pairing and registration
	NodeInfo   string
	UsePairing bool
}

type GWOptions struct {
	Registry *directory.Registry
	SRpcOpts []server.Option
	Options
}

type PeerOptions struct {
	SRpcOpts []client.Option
	Options
}
