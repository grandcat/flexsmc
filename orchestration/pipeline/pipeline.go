package pipeline

import (
	"github.com/grandcat/flexsmc/directory"
	proto "github.com/grandcat/flexsmc/proto"
)

type ProtoStack struct {
	Phases       []*proto.SMCCmd
	Participants []*directory.PeerInfo
}

type Pipe interface {
	Process(task *proto.SMCTask, inOut *ProtoStack) error
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

func (p *Pipeline) Process(task *proto.SMCTask) (phases []*proto.SMCCmd, peers []*directory.PeerInfo, err error) {
	changeset := &ProtoStack{}
	for _, pipe := range p.pipes {
		err = pipe.Process(task, changeset)
		if err != nil {
			return
		}
	}

	phases = changeset.Phases
	peers = changeset.Participants
	return
}
