package pipeline

import (
	"github.com/grandcat/flexsmc/directory"
	proto "github.com/grandcat/flexsmc/proto"
)

type OnlineFilter struct {
	Reg *directory.Registry
}

func (o *OnlineFilter) Process(task *proto.SMCTask, inOut *ProtoStack) error {
	// XXX: use all available (online) peers for now
	inOut.Participants = o.Reg.Watcher.AvailablePeers()
	return nil
}
