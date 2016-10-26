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
	parties := g.Reg.GetAll()
	inOut.Participants = make(map[directory.ChannelID]*directory.PeerInfo, len(parties))
	for idx, p := range parties {
		// The channelID is not important yet. Will be changed during a later pipe.
		inOut.Participants[directory.ChannelID(idx+1)] = p
	}

	return nil
}
