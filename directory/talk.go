package directory

import (
	proto "github.com/grandcat/flexsmc/proto"
)

type ChatWithPeer interface {
	Instruct() chan<- *proto.SMCCmd
	GetFeedback() <-chan *proto.CmdResult
}

type ChatWithGateway interface {
	GetInstructions() <-chan *proto.SMCCmd
	Feedback() chan<- *proto.CmdResult
}

type smcChat struct {
	// Channel to send some instructions to a certain peer
	to chan *proto.SMCCmd
	// Channel to listen for feedback from the same peer
	from chan *proto.CmdResult
}

func newTalker() smcChat {
	return smcChat{
		// Blocking channel: a peer must withdraw a chat request from
		// the gateway to establish a common communication channel.
		to:   make(chan *proto.SMCCmd),
		from: make(chan *proto.CmdResult),
	}
}

// For a gateway talking to single peer

func (t smcChat) Instruct() chan<- *proto.SMCCmd {
	return t.to
}

func (t smcChat) GetFeedback() <-chan *proto.CmdResult {
	return t.from
}

func (t smcChat) Close() {
	close(t.to)
}

// For a peer interacting with a gateway

func (t smcChat) GetInstructions() <-chan *proto.SMCCmd {
	return t.to
}

func (t smcChat) Feedback() chan<- *proto.CmdResult {
	return t.from
}
