package orchestration

import (
	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/flexsmc/logs"
	"github.com/grandcat/flexsmc/orchestration/aggregation"
	"github.com/grandcat/flexsmc/orchestration/pipeline"
	"github.com/grandcat/flexsmc/orchestration/worker"
	pbJob "github.com/grandcat/flexsmc/proto/job"
	"golang.org/x/net/context"
)

type Orchestration interface {
	Request(ctx context.Context, task *pbJob.SMCTask) (*pbJob.SMCResult, error)
}

// FifoOrchestration is thread-safe.
type FifoOrchestration struct {
	reg      *directory.Registry
	worker   *worker.PeerNetwork
	prePipe  *pipeline.Pipeline
	postAggr *aggregation.Aggregator
}

func NewFIFOOrchestration(reg *directory.Registry) Orchestration {
	// Init preprocessing pipeline
	pipe0 := &pipeline.GroupMap{Reg: reg}
	pipe1 := &pipeline.OnlineFilter{Reg: reg}
	pipe2 := &pipeline.ContinousChanID{}
	pipe3 := &pipeline.PhaseBuilder{}
	processInput := pipeline.NewPipeline(pipe0, pipe1, pipe2, pipe3)

	return &FifoOrchestration{
		reg:      reg,
		worker:   worker.NewPeerNetwork(reg),
		prePipe:  processInput,
		postAggr: new(aggregation.Aggregator),
	}
}

func (fo *FifoOrchestration) Request(ctx context.Context, task *pbJob.SMCTask) (*pbJob.SMCResult, error) {
	// 1. Transform task to set of instructions (should not block)
	jobInstr, err := fo.prePipe.Process(task)
	if err != nil {
		logs.I.Infof("Orchestration: preprocess pipeline failed: %v", err.Error())
		return nil, err
	}
	// 2. Submit job to worker pool
	logs.VV.Infoln("Pipeline peers:", jobInstr.Participants)
	logs.VV.Infoln("Pipeline phases:", jobInstr.Tasks)
	job, err := fo.worker.SubmitJob(ctx, *jobInstr, worker.HandleErrFlags(pbJob.CmdResult_ERR_CLASS_NORM|pbJob.CmdResult_ERR_CLASS_COMM))
	if err != nil {
		logs.I.Infof("Orchestration: job submission failed: %v", err.Error())
		return nil, err
	}
	// 3. Wait for ingress, aggregate data and do reasoning
	res, err := fo.postAggr.Process(ctx, job)

	// XXX: 4. Try rescheduling job, but excluding the errorneous peers for now
	if err != nil {
		logs.V.Infof("PREV RESULT BEFORE RESUBMIT: %v [Error: %v]", res, err)
		// Let's assume a peer dropped its connection. So just rerun the job pipeline
		// should suffice in most cases. Let's try :)
		newJobInstr, _ := fo.prePipe.Process(task)
		// If participants differ in both instruction sets, our assumption is correct and
		// we can continue. Otherwise, something else causes a problem. Then, it is not
		// useful resubmitting the job as it will fail again immediately.
		if len(newJobInstr.Participants) < len(jobInstr.Participants) {
			logs.I.Infof("Job seems to differ, so start resubmit")
			logs.V.Infof("New parties: %v", newJobInstr.Participants)
			err = fo.worker.RescheduleOpenJob(ctx, job, *newJobInstr, worker.HandleErrFlags(0))
			if err != nil {
				logs.I.Infof("Orchestration: job resubmit failed: %v", err.Error())
				return nil, err
			}
			res, err = fo.postAggr.Process(ctx, job)
			if err != nil {
				// Should not be necessary to abort manually as every error should abort the
				// complete job due to worker.HandleErrFlags(0) .
				job.Abort()
			}

		} else {
			aerr := job.Abort()
			logs.VV.Infof("Orchestration !!Abort: %v", aerr)
		}
	}
	return res, err
}
