package worker

import "github.com/grandcat/flexsmc/directory"
import "fmt"
import "bytes"

// JobImplication describes the state a job is left.
//go:generate stringer -type=JobImplication
type JobImplication uint8

const (
	Unknown JobImplication = iota
	Halted
	Finished
	Aborted
)

type JobError struct {
	status JobImplication
	err    error
}

func (e *JobError) Error() string {
	return fmt.Sprintf("%s: s->%s", e.err.Error(), e.status.String())
}

func (e *JobError) Status() JobImplication {
	return e.status
}

type PeerError struct {
	JobError
	progress JobPhase
	peers    []*directory.PeerInfo
}

func newPeerErr(e error, status JobImplication, progress JobPhase, peers []*directory.PeerInfo) *PeerError {
	return &PeerError{
		progress: progress,
		peers:    peers,
		JobError: JobError{
			err:    e,
			status: status,
		},
	}
}

func (e *PeerError) Error() string {
	var msg bytes.Buffer
	msg.WriteString(e.err.Error())
	msg.WriteString(" [")

	for i, p := range e.peers {
		msg.WriteString(string(p.ID))
		if i != (len(e.peers) - 1) {
			msg.WriteByte(' ')
		}
	}
	msg.WriteString("]")
	return msg.String()
}

func (e *PeerError) Progress() JobPhase {
	return e.progress
}

func (e *PeerError) AffectedPeers() []*directory.PeerInfo {
	return e.peers
}
