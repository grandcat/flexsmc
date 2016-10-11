package orchestration

import (
	"log"

	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/flexsmc/orchestration/aggregation"
	"github.com/grandcat/flexsmc/orchestration/pipeline"
	"github.com/grandcat/flexsmc/orchestration/worker"
	proto "github.com/grandcat/flexsmc/proto"
	"golang.org/x/net/context"
)

type Orchestration interface {
	Request(ctx context.Context, task *proto.SMCTask) (*proto.SMCResult, error)
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
	pipe1 := &pipeline.OnlineFilter{Reg: reg}
	pipe2 := &pipeline.PhaseBuilder{}
	processInput := pipeline.NewPipeline(pipe1, pipe2)

	return &FifoOrchestration{
		reg:      reg,
		worker:   worker.NewPeerNetwork(reg),
		prePipe:  processInput,
		postAggr: new(aggregation.Aggregator),
	}
}

func (fo *FifoOrchestration) Request(ctx context.Context, task *proto.SMCTask) (*proto.SMCResult, error) {
	// 1. Transform task to set of instructions (shuld not block)
	jobInstr, err := fo.prePipe.Process(task)
	if err != nil {
		log.Printf("Orchestration: preprocess pipeline failed: %v", err.Error())
		return nil, err
	}
	// 2. Submit job to worker pool
	log.Println(">> Pipeline peers:", jobInstr.Participants)
	log.Println(">> Pipeline phases:", jobInstr.Tasks)
	job, err := fo.worker.SubmitJob(ctx, *jobInstr)
	if err != nil {
		log.Printf("Orchestration: job submission failed: %v", err.Error())
		return nil, err
	}
	// 3. Wait for ingress, aggregate data and do reasoning
	res, err := fo.postAggr.Process(ctx, job)
	return res, err
}
