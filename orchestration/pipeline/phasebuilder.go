package pipeline

import (
	"errors"

	"github.com/grandcat/flexsmc/directory"
	proto "github.com/grandcat/flexsmc/proto"
)

type PhaseBuilder struct {
	Reg *directory.Registry
}

func (b *PhaseBuilder) Process(task *proto.SMCTask, inOut *ProtoStack) error {
	if len(inOut.Phases) > 0 {
		return errors.New("expect no SMC phases before running PhaseBuilder")
	}

	// Prepare phase
	var participants []*proto.Prepare_Participant
	for _, p := range inOut.Participants {
		participants = append(participants, &proto.Prepare_Participant{Identity: string(p.ID), Addr: p.Addr.String()})
	}

	p1 := &proto.SMCCmd{
		State: proto.SMCCmd_PREPARE,
		Payload: &proto.SMCCmd_Prepare{Prepare: &proto.Prepare{
			SmcTask:      task,
			Participants: participants,
		}},
	}

	// Session phase
	p2 := &proto.SMCCmd{
		State:   proto.SMCCmd_SESSION,
		Payload: &proto.SMCCmd_Session{Session: &proto.SessionPhase{}},
	}

	inOut.Phases = append(inOut.Phases, p1, p2)
	return nil
}
