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

	var numPingPhases int
	switch task.Aggregator {
	case pbJob.Aggregator_DBG_PINGPONG_10:
		numPingPhases = 10
	case pbJob.Aggregator_DBG_PINGPONG_100:
		numPingPhases = 100
	default:
		numPingPhases = 1
	}

	for i := numPingPhases; i > 0; i-- {
		ph := &pbJob.SMCCmd{
			SessionID: sessID,
			Payload: &pbJob.SMCCmd_Debug{Debug: &pbJob.DebugPhase{
				Ping:       int32(i),
				MorePhases: (i > 1),
				Options:    task.Options,
			}},
		}
		inOut.Tasks = append(inOut.Tasks, ph)
	}

	return nil
}
