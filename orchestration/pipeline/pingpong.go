package pipeline

import (
	"errors"

	"github.com/grandcat/flexsmc/orchestration/worker"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

type DbgPingPong struct {
}

func (pp *DbgPingPong) Process(task *pbJob.SMCTask, inOut *worker.JobInstruction) error {
	if len(inOut.Tasks) > 0 {
		return errors.New("expect no SMC phases before running PhaseBuilder")
	}

	// TODO: generate unique session ID
	sessID := "f13xdbg"

	// Debug phase
	p1 := &pbJob.SMCCmd{
		SessionID: sessID,
		Payload: &pbJob.SMCCmd_Debug{Debug: &pbJob.DebugPhase{
			Ping:    42,
			Options: task.Options,
		}},
	}

	inOut.Tasks = append(inOut.Tasks, p1)
	return nil
}
