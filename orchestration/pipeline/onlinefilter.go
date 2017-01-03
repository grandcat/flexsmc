package pipeline

import (
	"fmt"

	"math"

	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/flexsmc/orchestration/worker"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

type OnlineFilter struct {
	Reg *directory.Registry
}

func (o *OnlineFilter) Process(task *pbJob.SMCTask, inOut *worker.JobInstruction) error {
	var minParties int32 = 1
	var maxParties = int32(math.MaxInt32)
	// Required number of participants can be overwritten by option arguments.
	if tOpt, ok := task.Options["minNumPeers"]; ok && tOpt.GetDec() > 0 {
		minParties = tOpt.GetDec()
	}
	if tOpt, ok := task.Options["maxNumPeers"]; ok && tOpt.GetDec() >= minParties {
		maxParties = tOpt.GetDec()
	}
	// Filter list of participants in-place.
	peersOn := o.Reg.Watcher.AvailablePeers()

	var cntSelPeers int32
	for cid, p := range inOut.Participants {
		if _, online := peersOn[p.ID]; !online || cntSelPeers >= maxParties {
			// Ignore peer as it is either offline or exceeds amount of participants.
			delete(inOut.Participants, cid)
		} else {
			// Peer selected to participate in this round.
			cntSelPeers++
		}
	}
	// Check if enough participants are left over if required.
	if minParties > int32(len(inOut.Participants)) {
		return fmt.Errorf("OnlineFilter: requested number of peers %d > %d peers online",
			minParties, len(inOut.Participants))
	}
	return nil
}
