package directory

import pbJob "github.com/grandcat/flexsmc/proto/job"

type ChatWithPeer interface {
	Peer() *PeerInfo
	Instruct() chan<- *pbJob.SMCCmd
	GetFeedback() <-chan *pbJob.CmdResult
	Close()
}

type ChatWithGateway interface {
	GetInstructions() <-chan *pbJob.SMCCmd
	Feedback() chan<- *pbJob.CmdResult
	// Metadata handling
	SetPeerMetadata(r *pbJob.CmdResult)
	SetMetadata(r *pbJob.CmdResult, key, value string)
}

// 0 = Blocking channel:
// Both peer and GW must be active at the same time to collaboratively
// exchange messages with each other. This might hinder other peers
// ready for message transfer.
// 1 <= Buffered channel:
// Both peer and GW deliver messages asynchronously.
const chatBufLen = 1

type smcChat struct {
	peer *PeerInfo
	// Channel to send some instructions to a certain peer
	to chan *pbJob.SMCCmd
	// Channel to listen for feedback from the same peer
	from chan *pbJob.CmdResult
}

func newTalker(p *PeerInfo) smcChat {
	return smcChat{
		peer: p,
		to:   make(chan *pbJob.SMCCmd, chatBufLen),
		from: make(chan *pbJob.CmdResult, chatBufLen),
	}
}

// For a Gateway talking to single peer

func (t smcChat) Peer() *PeerInfo {
	return t.peer
}

func (t smcChat) Instruct() chan<- *pbJob.SMCCmd {
	return t.to
}

func (t smcChat) GetFeedback() <-chan *pbJob.CmdResult {
	return t.from
}

func (t smcChat) Close() {
	close(t.to)
}

// For a Peer interacting with a gateway

func (t smcChat) GetInstructions() <-chan *pbJob.SMCCmd {
	return t.to
}

func (t smcChat) Feedback() chan<- *pbJob.CmdResult {
	return t.from
}

func (t smcChat) SetPeerMetadata(r *pbJob.CmdResult) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
	r.Metadata["peerID"] = string(t.peer.ID)
}

func (t smcChat) SetMetadata(r *pbJob.CmdResult, key, value string) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
	r.Metadata[key] = value
}
