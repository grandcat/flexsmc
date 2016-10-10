package pipeline

import (
	"github.com/grandcat/flexsmc/orchestration"
	proto "github.com/grandcat/flexsmc/proto"
)

type Pipe interface {
	Process(task *proto.SMCTask, inOut *orchestration.JobInstruction) error
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

func (p *Pipeline) Process(task *proto.SMCTask) (*orchestration.JobInstruction, error) {
	changeset := &orchestration.JobInstruction{}
	for _, pipe := range p.pipes {
		err := pipe.Process(task, changeset)
		if err != nil {
			return nil, err
		}
	}
	return changeset, nil
}
