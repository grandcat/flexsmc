package worker

import "github.com/grandcat/flexsmc/directory"

// JobImplication describes the state a job is left.
//go:generate stringer -type=JobImplication
type JobImplication uint8

const (
	Unknown JobImplication = iota
	Halted
	Finished
	Aborted
)

type PeerError struct {
	Status   JobImplication
	Progress JobPhase
	Peers    []*directory.PeerInfo
	Err      error
}

func NewPeerErr(e error, status JobImplication, progress JobPhase, peers []*directory.PeerInfo) *PeerError {
	return &PeerError{
		Err:      e,
		Status:   status,
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
	msg += "] "
	return msg + "? -> " + e.Status.String()
}

func (e *PeerError) AffectedPeers() []*directory.PeerInfo {
	return e.Peers
}
