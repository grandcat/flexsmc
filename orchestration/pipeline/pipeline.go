package pipeline

import (
	"github.com/grandcat/flexsmc/orchestration/worker"
	proto "github.com/grandcat/flexsmc/proto"
)

type Pipe interface {
	Process(task *proto.SMCTask, inOut *worker.JobInstruction) error
}

type Pipeline struct {
	pipes []Pipe
}

func NewPipeline(pipes ...Pipe) *Pipeline {
	p := &Pipeline{
		pipes: pipes,
	}
	return p
}

func (p *Pipeline) Process(task *proto.SMCTask) (*worker.JobInstruction, error) {
	changeset := &worker.JobInstruction{}
	for _, pipe := range p.pipes {
		err := pipe.Process(task, changeset)
		if err != nil {
			return nil, err
		}
	}
	return changeset, nil
}
