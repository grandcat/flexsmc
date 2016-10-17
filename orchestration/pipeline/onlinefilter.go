package pipeline

import (
	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/flexsmc/orchestration/worker"
	proto "github.com/grandcat/flexsmc/proto"
)

type OnlineFilter struct {
	Reg *directory.Registry
}

func (o *OnlineFilter) Process(task *proto.SMCTask, inOut *worker.JobInstruction) error {
	peersOn := o.Reg.Watcher.AvailablePeers()
	// Filter list of participants in-place
	filteredPeers := inOut.Participants[:0]
	for _, p := range inOut.Participants {
		if _, ok := peersOn[p.ID]; ok {
			filteredPeers = append(filteredPeers, p)
		}
	}
	inOut.Participants = filteredPeers

	return nil
}
