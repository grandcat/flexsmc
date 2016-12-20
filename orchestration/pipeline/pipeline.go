package pipeline

import (
	"github.com/grandcat/flexsmc/orchestration/worker"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

type Pipe interface {
	Process(task *pbJob.SMCTask, inOut *worker.JobInstruction) error
}

type Pipeline struct {
	pipes []Pipe

	// Optional
	dgbPipes []Pipe
}

func NewPipeline(pipes ...Pipe) *Pipeline {
	p := &Pipeline{
		pipes: pipes,
	}
	return p
}

func (p *Pipeline) Process(task *pbJob.SMCTask) (*worker.JobInstruction, error) {
	changeset := &worker.JobInstruction{}
	selectedPipes := p.pipes
	// Use different pipes for debug aggregators in DEBUG mode.
	if task.Aggregator >= pbJob.Aggregator_DBG_PINGPONG && len(p.dgbPipes) > 0 {
		selectedPipes = p.dgbPipes
	}
	// Process.
	for _, pipe := range selectedPipes {
		err := pipe.Process(task, changeset)
		if err != nil {
			return nil, err
		}
	}
	return changeset, nil
}

func (p *Pipeline) DedicatedDebugPipes(pipes ...Pipe) {
	p.dgbPipes = pipes
}
