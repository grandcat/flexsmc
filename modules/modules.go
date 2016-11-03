package modules

import (
	"sync"

	proto "github.com/grandcat/flexsmc/proto"
	"golang.org/x/net/context"
)

type ModuleContext struct {
	context    context.Context
	ActiveMods *sync.WaitGroup //< notify master routine when done due to error or ctx
	faults     chan error
	// Peer interfacing with gateway
	GWConn proto.GatewayClient
}

func NewModuleContext(ctx context.Context, gwConn proto.GatewayClient) ModuleContext {
	return ModuleContext{
		context:    ctx,
		GWConn:     gwConn,
		ActiveMods: new(sync.WaitGroup),
		faults:     make(chan error),
	}
}

func (m ModuleContext) reportFault(err error) {
	select {
	case m.faults <- err:
		// Submitted successfully
	case <-m.context.Done():
		// In case another module caused canceling all modules, the fault channel
		// might not be checked anymore. So it would block the calling module.
		// Release this go routine here to prevent such a situation.
	}
}

func (m ModuleContext) Faults() <-chan error {
	return m.faults
}

func (m ModuleContext) WaitAll() {
	m.ActiveMods.Wait()
}
