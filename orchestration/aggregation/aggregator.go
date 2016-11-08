package aggregation

import (
	"errors"

	"github.com/golang/glog"
	"github.com/grandcat/flexsmc/orchestration/worker"
	pbJob "github.com/grandcat/flexsmc/proto/job"
	"golang.org/x/net/context"
)

var ErrEmptyJob = errors.New("nothing to aggregate due to missing job input")
var ErrInconsistentResult = errors.New("inconsistent peer results")

type Aggregator struct {
}

func (a *Aggregator) Process(ctx context.Context, in worker.JobWatcher) (*pbJob.SMCResult, error) {
	numParticipants := len(in.Job().Participants)
	msgBuf := make([][]*pbJob.CmdResult, len(in.Job().Tasks))

	var lastProgress worker.JobPhase = -1
loop:
	for {
		select {
		case res, ok := <-in.Result():
			if !ok {
				err := in.Err()
				glog.V(3).Infoln("End of stream with error:", err)
				if err != nil {
					// XXX: return anonymous result only (without details about peers)?
					return nil, err
				}
				// All messages complete
				break loop
			}

			// Transition to new phase
			if lastProgress < res.Progress {
				// Process complete phase previously recorded (if any)
				if lastProgress >= 0 {
					if err := analyzeResultConsistency(msgBuf[lastProgress]); err != nil {
						return nil, err
					}
				}
				// Pre-allocate buffer for next phase
				msgBuf[int(res.Progress)] = make([]*pbJob.CmdResult, 0, numParticipants)
				lastProgress = res.Progress
			}

			glog.V(3).Infof("Received %v", res.Response)
			msgBuf[int(res.Progress)] = append(msgBuf[int(res.Progress)], res.Response)

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if lastProgress < 0 {
		return nil, ErrEmptyJob
	}

	// Final phase (XXX: session phase and so the interesting one for testing purpose)
	if err := analyzeResultConsistency(msgBuf[lastProgress]); err != nil {
		return nil, err
	}
	return msgBuf[lastProgress][0].Result, nil
}

// XXX: also compare with job. It could be that some peers are not intended to send some
// numeric feedback. This might be extracted from the SMCCmds in future.
// In case of some peers sending a different value, it might also be worth to accept the
// most certain result if num of different values is not about a critical threshold compared
// to participating peers.
// XXX: very hacky function. The whole aggregator needs a more proper design to work for
// more generic tasks.
func analyzeResultConsistency(msgs []*pbJob.CmdResult) error {
	if len(msgs) == 0 {
		return nil
	}

	glog.V(3).Infoln("First entry:", msgs[0].Result)

	// nil test
	if msgs[0].Result == nil {
		for _, m := range msgs {
			if m.Result != nil {
				return ErrInconsistentResult
			}
		}
	}
	// Compare all struct members
	if msgs[0].Result != nil {
		res := *msgs[0].Result
		for _, m := range msgs {
			if m.Result == nil {
				return ErrInconsistentResult
			}
			if res != *m.Result {
				return ErrInconsistentResult
			}
		}
	}

	return nil
}
