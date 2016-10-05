package modules

import (
	"log"
	"time"

	proto "github.com/grandcat/flexsmc/proto"
)

type HealthReporter struct {
	ModuleContext
	PingInterval time.Duration
}

func NewHealthReporter(modInfo ModuleContext, pingInterval time.Duration) *HealthReporter {
	return &HealthReporter{
		ModuleContext: modInfo,
		PingInterval:  pingInterval,
	}
}

func (h *HealthReporter) Start() {
	h.ActiveMods.Add(1)
	go h.report()
}

func (h *HealthReporter) report() {
	defer h.ActiveMods.Done()

	ticker := time.NewTicker(h.PingInterval)
	defer ticker.Stop()

	for {
		// Ping
		resp, err := h.GWConn.Ping(h.Context, &proto.SMCInfo{12345})
		if err != nil {
			log.Printf("Could not ping GW: %v", err)
			return
		}
		log.Println("Ping resp:", resp.Status)

		// Schedule next or abort
		select {
		case <-ticker.C:
			// Do nothing and ping again on next round :-)

		case <-h.Context.Done():
			// Abort by context
			log.Printf("Health reporter aborted: %v", h.Context.Err())
			return
		}
	}
}
