package pipeline

import (
	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/flexsmc/orchestration/worker"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

// GroupMap maps group entities to a set of responsible peers.
// XXX: the current implementation assumes just a single default group.
//		Therefore, it just maps all peers from the directory.
type GroupMap struct {
	Reg *directory.Registry
}

func (g *GroupMap) Process(task *pbJob.SMCTask, inOut *worker.JobInstruction) error {
	inOut.Participants = g.Reg.GetAll()
	return nil
}
