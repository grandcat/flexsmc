package worker

import "github.com/grandcat/flexsmc/directory"

type PeerError struct {
	Progress JobPhase
	Peers    []*directory.PeerInfo
	Err      error
}

func NewPeerErr(e error, progress JobPhase, peers []*directory.PeerInfo) *PeerError {
	return &PeerError{
		Err:      e,
		Progress: progress,
		Peers:    peers,
	}
}

func (e *PeerError) Error() string {
	msg := e.Err.Error() + " ["
	for i, p := range e.Peers {
		msg += string(p.ID)
		if i != (len(e.Peers) - 1) {
			msg += " "
		}
	}
	return msg + "]"
}

func (e *PeerError) AffectedPeers() []*directory.PeerInfo {
	return e.Peers
}
