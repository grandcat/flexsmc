package pipeline

import (
	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/flexsmc/orchestration/worker"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

type OnlineFilter struct {
	Reg *directory.Registry
}

func (o *OnlineFilter) Process(task *pbJob.SMCTask, inOut *worker.JobInstruction) error {
	peersOn := o.Reg.Watcher.AvailablePeers()
	// Filter list of participants in-place
	for cid, p := range inOut.Participants {
		if _, ok := peersOn[p.ID]; !ok {
			delete(inOut.Participants, cid)
		}
	}
	return nil
}
