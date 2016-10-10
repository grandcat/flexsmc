package modules

import (
	"sync"

	proto "github.com/grandcat/flexsmc/proto"
	"golang.org/x/net/context"
)

type ModuleContext struct {
	Context    context.Context
	ActiveMods *sync.WaitGroup //< notify master routine when done due to error or ctx
	faults     chan error
	// Peer interfacing with gateway
	GWConn proto.GatewayClient
}

func NewModuleContext(ctx context.Context, gwConn proto.GatewayClient) ModuleContext {
	return ModuleContext{
		Context:    ctx,
		GWConn:     gwConn,
		ActiveMods: new(sync.WaitGroup),
		faults:     make(chan error),
	}
}

func (m ModuleContext) Faults() <-chan error {
	return m.faults
}

func (m ModuleContext) WaitAll() {
	m.ActiveMods.Wait()
}
