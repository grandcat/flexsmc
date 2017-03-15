package pipeline

import (
	"errors"

	"fmt"

	"github.com/grandcat/flexsmc/orchestration/worker"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

type PhaseBuilder struct {
	idCnt int
}

func (b *PhaseBuilder) Process(task *pbJob.SMCTask, inOut *worker.JobInstruction) error {
	if len(inOut.Tasks) > 0 {
		return errors.New("expect no SMC phases before running PhaseBuilder")
	}

	// TODO: generate unique session ID
	sessID := fmt.Sprintf("f13x%05d", b.idCnt)
	// b.idCnt++
	// XXX: keep it the same. It will verify whether resources are cleaned properly on peer side.

	// Collect configuration for Prepare phase
	var participants []*pbJob.PreparePhase_Participant
	for cid, p := range inOut.Participants {
		// XXX: for testing: use continous port range. Does not respect used ports
		//		on host site.
		//		We assume that a peer will complain right now if it does not work out.
		if p.Addr.Port < 10000 {
			p.Addr.Port = 10000 + int(cid)
		}
		participants = append(participants, &pbJob.PreparePhase_Participant{
			AuthID:    string(p.ID),
			SmcPeerID: int32(cid),
			Endpoint:  p.Addr.String(),
		})
	}

	// Prepare phase.
	p1 := &pbJob.SMCCmd{
		SessionID: sessID,
		State:     pbJob.SMCCmd_PREPARE,
		Payload: &pbJob.SMCCmd_Prepare{Prepare: &pbJob.PreparePhase{
			SmcTask:      task,
			Participants: participants,
		}},
	}
	inOut.Tasks = append(inOut.Tasks, p1)

	// Optional Linking phase.
	// Improves robustness with respect to early error handling, but
	// introduces a further round-trip to all involved peers.
	if _, ok := task.Options["useLinkingPhase"]; ok {
		p12 := &pbJob.SMCCmd{
			SessionID: sessID,
			State:     pbJob.SMCCmd_LINK,
			Payload:   &pbJob.SMCCmd_Link{Link: &pbJob.LinkingPhase{}},
		}
		inOut.Tasks = append(inOut.Tasks, p12)
		// // Not necessary to propagate to the peers as every peer will
		// // see the phase anyways.
		// // So, remove some bloat.
		// delete(task.Options, "useLinkingPhase")
	}

	// Session phase
	var batchOpSize int32
	if tOpt, ok := task.Options["batchOpSize"]; ok && tOpt.GetDec() > 0 {
		batchOpSize = tOpt.GetDec()
	}

	p2 := &pbJob.SMCCmd{
		SessionID: sessID,
		State:     pbJob.SMCCmd_SESSION,
		Payload: &pbJob.SMCCmd_Session{Session: &pbJob.SessionPhase{
			ParallelBatchOps: batchOpSize,
		}},
	}
	inOut.Tasks = append(inOut.Tasks, p2)

	return nil
}
