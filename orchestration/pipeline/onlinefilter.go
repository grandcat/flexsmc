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
	// XXX: use all available (online) peers for now
	inOut.Participants = o.Reg.Watcher.AvailablePeers()
	return nil
}
