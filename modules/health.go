package modules

import (
	"time"

	"github.com/golang/glog"
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
	go h.pingloop()
}

func (h *HealthReporter) pingloop() {
	defer h.ActiveMods.Done()

	ticker := time.NewTicker(h.PingInterval)
	defer ticker.Stop()

	for {
		// Ping
		if err := h.Ping(); err != nil {
			return
		}
		// Schedule next or abort
		select {
		case <-ticker.C:
			// Do nothing and ping again on next round :-)

		case <-h.context.Done():
			// Abort by context
			glog.Warningf("Health reporter aborted: %v", h.context.Err())
			return
		}
	}
}

func (h *HealthReporter) Ping() error {
	resp, err := h.GWConn.Ping(h.context, &proto.SMCInfo{12345})
	if err != nil {
		glog.V(1).Infof("Could not ping GW: %v", err)
		return err
	}
	glog.V(3).Infoln("Ping resp:", resp.Status)
	return nil
}
