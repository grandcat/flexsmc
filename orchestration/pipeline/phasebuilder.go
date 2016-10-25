package pipeline

import (
	"errors"

	"github.com/grandcat/flexsmc/orchestration/worker"
	proto "github.com/grandcat/flexsmc/proto"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

type PhaseBuilder struct {
}

func (b *PhaseBuilder) Process(task *pbJob.SMCTask, inOut *worker.JobInstruction) error {
	if len(inOut.Tasks) > 0 {
		return errors.New("expect no SMC phases before running PhaseBuilder")
	}

	// Prepare phase
	var participants []*pbJob.PreparePhase_Participant
	for i, p := range inOut.Participants {
		participants = append(participants, &pbJob.PreparePhase_Participant{
			AuthID:    string(p.ID),
			SmcPeerID: int32(i + 1),
			Endpoint:  p.Addr.String(),
		})
	}

	p1 := &proto.SMCCmd{
		State: proto.SMCCmd_PREPARE,
		Payload: &proto.SMCCmd_Prepare{Prepare: &pbJob.PreparePhase{
			SmcTask:      task,
			Participants: participants,
			// SmcPeerID is filled by job worker
		}},
	}

	// Session phase
	p2 := &proto.SMCCmd{
		State:   proto.SMCCmd_SESSION,
		Payload: &proto.SMCCmd_Session{Session: &pbJob.SessionPhase{}},
	}

	inOut.Tasks = append(inOut.Tasks, p1, p2)
	return nil
}
