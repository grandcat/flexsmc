package pipeline

import (
	"errors"

	"github.com/grandcat/flexsmc/orchestration/worker"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

type PhaseBuilder struct {
}

func (b *PhaseBuilder) Process(task *pbJob.SMCTask, inOut *worker.JobInstruction) error {
	if len(inOut.Tasks) > 0 {
		return errors.New("expect no SMC phases before running PhaseBuilder")
	}

	// TODO: generate unique session ID
	sessID := "f13x0123456789"

	// Prepare phase
	var participants []*pbJob.PreparePhase_Participant
	for cid, p := range inOut.Participants {
		participants = append(participants, &pbJob.PreparePhase_Participant{
			AuthID:    string(p.ID),
			SmcPeerID: int32(cid),
			Endpoint:  p.Addr.String(),
		})
	}

	p1 := &pbJob.SMCCmd{
		SessionID: sessID,
		State:     pbJob.SMCCmd_PREPARE,
		Payload: &pbJob.SMCCmd_Prepare{Prepare: &pbJob.PreparePhase{
			SmcTask:      task,
			Participants: participants,
			// SmcPeerID is filled by job worker
		}},
	}

	// Session phase
	p2 := &pbJob.SMCCmd{
		SessionID: sessID,
		State:     pbJob.SMCCmd_SESSION,
		Payload:   &pbJob.SMCCmd_Session{Session: &pbJob.SessionPhase{}},
	}

	inOut.Tasks = append(inOut.Tasks, p1, p2)
	return nil
}
