package pipeline

import (
	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/flexsmc/orchestration/worker"
	proto "github.com/grandcat/flexsmc/proto"
)

// GroupMap maps group entities to a set of responsible peers.
// XXX: the current implementation assumes just a single default group.
//		Therefore, it just maps all peers from the directory.
type GroupMap struct {
	Reg *directory.Registry
}

func (g *GroupMap) Process(task *proto.SMCTask, inOut *worker.JobInstruction) error {
	inOut.Participants = g.Reg.GetAll()
	return nil
}
