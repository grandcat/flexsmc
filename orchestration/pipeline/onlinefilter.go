package pipeline

import (
	"fmt"

	"math"

	"github.com/golang/glog"
	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/flexsmc/orchestration/worker"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

type OnlineFilter struct {
	Reg   *directory.Registry
	Debug bool
}

func (o *OnlineFilter) Process(task *pbJob.SMCTask, inOut *worker.JobInstruction) error {
	// Just use default chronological order without respect to PeerID (like groupmap).
	var participants = directory.ParticipantsToList(inOut.Participants)
	var minParties int32 = 1
	var maxParties = int32(math.MaxInt32)
	if o.Debug {
		// Required number of participants can be overwritten by option arguments.
		if tOpt, ok := task.Options["minNumPeers"]; ok && tOpt.GetDec() > 0 {
			minParties = tOpt.GetDec()
		}
		if tOpt, ok := task.Options["maxNumPeers"]; ok && tOpt.GetDec() >= minParties {
			maxParties = tOpt.GetDec()
		}
		// Sort peer IDs to produce deterministic set of nodes.
		// For benchmarking, we need the same specific nodes and communication paths if
		// the same task is submitted multiple times. This should result in comparable
		// results if sampled long enough.
		if tOpt, ok := task.Options["sortPeerIDs"]; ok && tOpt.GetDec() > 0 {
			participants = participants.SortbyPeerID()
		}
	}

	// Filter list of participants in-place.
	peersOn := o.Reg.Watcher.AvailablePeers()

	var cntSelPeers int32
	for _, p := range participants {
		if _, online := peersOn[p.PeerID]; !online || cntSelPeers >= maxParties {
			// Ignore peer as it is either offline or exceeds amount of participants.
			delete(inOut.Participants, p.ChanID)
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

	if glog.V(3) {
		glog.Info("Filtered participants: ")
		for _, p := range inOut.Participants {
			glog.Info(p.ID, " ")
		}
	}

	return nil
}
