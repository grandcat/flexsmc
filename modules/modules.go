package modules

import (
	"sync"

	proto "github.com/grandcat/flexsmc/proto"
	"golang.org/x/net/context"
)

type ModuleContext struct {
	Context    context.Context
	ActiveMods *sync.WaitGroup //< notify master routine when done due to error or ctx
	// Peer interfacing with gateway
	GWConn proto.GatewayClient
}

func (m ModuleContext) UpdateConnection(c proto.GatewayClient) {
	m.GWConn = c
}
