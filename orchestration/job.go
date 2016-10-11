package orchestration

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/grandcat/flexsmc/directory"
	proto "github.com/grandcat/flexsmc/proto"
	auth "github.com/grandcat/srpc/authentication"
)

type JobInstruction struct {
	Tasks        []*proto.SMCCmd
	Participants []*directory.PeerInfo
}

type JobWatcher interface {
	// Job returns the instruction associated with this job.
	Job() JobInstruction
	// Result is a stream of responses sent by their peers.
	// To differentiate multiple protocol phases, each result carries
	// the current progress.
	// Progress must NOT mix. This means there is an unique transition from
	// phase 0 -> phase 1, for instance.
	Result() <-chan PeerResult
	// Err is non-nil if a critical error occurred during operation.
	// It should be called first when Result() chan was called from our side.
	Err() *PeerError
}

type TaskPhase int

type PeerResult struct {
	Progress TaskPhase
	Response *proto.CmdResult
}

type job struct {
	feedback chan PeerResult
	lastErr  *PeerError
	mu       sync.Mutex
	// Instruction with job context, target peers and their task to do
	ctx   context.Context
	instr JobInstruction
	// Context for worker processing this job
	progress TaskPhase
	chats    map[auth.PeerID]directory.ChatWithPeer
}

func newJob(ctx context.Context, instruction JobInstruction) *job {
	return &job{
		feedback: make(chan PeerResult),
		ctx:      ctx,
		instr:    instruction,
	}
}

func (j *job) Job() JobInstruction {
	return j.instr
}

func (j *job) Result() <-chan PeerResult {
	return j.feedback
}

func (j *job) Err() *PeerError {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.lastErr
}

// API facing communication with worker peers

// openPeerChats initiates a bi-directional communication channel to each peer.
// If it takes too much time until the corresponding peer reacts on our
// talk request, it is dropped and the job informs the originator about that.
//
// For a resubmitted job, established chats are kept alive for peers whose
// communication worked. Faulty chats are kicked on any failure on the
// communication layer.
func (j *job) openPeerChats(ctx context.Context) *PeerError {
	var errPeers []*directory.PeerInfo

	if j.chats == nil {
		j.chats = make(map[auth.PeerID]directory.ChatWithPeer, len(j.instr.Participants))
	}

	for _, p := range j.instr.Participants {
		if _, exists := j.chats[p.ID]; exists {
			continue
		}

		pch, err := p.RequestChat(ctx)
		if err != nil {
			errPeers = append(errPeers, p)
			log.Printf("[%s] Talk request not handled fast enough. Aborting.", p.ID)
			continue
		}
		j.chats[p.ID] = pch
	}

	if len(errPeers) > 0 {
		return NewPeerErr(ctx.Err(), j.progress, errPeers)
	}
	return nil
}

// removePeerChatdeletes the chats to these peers.
// It is essential to do so that a resubmitted job has a new chance to
// initiate a new chat to a previously faulty peer.
func (j *job) removePeerChats(peers []*directory.PeerInfo) {
	for _, p := range peers {
		j.chats[p.ID].Close()
		// delete(j.chats, p.ID)
	}
}

func (j *job) closeAllChats() {
	for _, ch := range j.chats {
		ch.Close()
	}
	j.chats = nil
}

var errCtxOrStreamFailure = errors.New("ctx timeout or stream failure")

func (j *job) queryTargetsSync(ctx context.Context, cmd *proto.SMCCmd) ([]*proto.CmdResult, *PeerError) {
	// First, disseminate the job to all peers.
	// Then, collect all results, but expect the results to be there until timeout occurs.

	// Send
	for _, pch := range j.chats {
		pch.Instruct() <- cmd
	}
	// Receive
	// Each peer delivers its response independently from each other. If one peer blocks,
	// there is still the result of all other peers after the timeout occurs.
	var errPeers []*directory.PeerInfo
	resps := make([]*proto.CmdResult, 0, len(j.chats))
	for _, pch := range j.chats {
		resp, err := pullRespUntilDone(ctx, pch.GetFeedback())
		switch {
		case err != nil:
			fallthrough
		case resp.Status >= proto.CmdResult_STREAM_ERR:
			errPeers = append(errPeers, pch.Peer())
			log.Printf("[%s] job communication failed: %v", pch.Peer().ID, err)
			pch.Close()
			continue
		}
		resps = append(resps, resp)
		log.Printf(">>[%s] Response from peer: %v", pch.Peer().ID, resp)
	}

	var err *PeerError
	if len(errPeers) > 0 {
		j.removePeerChats(errPeers)
		err = NewPeerErr(errCtxOrStreamFailure, j.progress, errPeers)
	}
	return resps, err
}

var ErrEmptyChannel = errors.New("input channel is empty")

func pullRespUntilDone(ctx context.Context, in <-chan *proto.CmdResult) (*proto.CmdResult, error) {
	// Context timeout or abort: only collect result if available immediately
	if ctx.Err() != nil {
		select {
		case resp := <-in:
			return resp, nil
		default:
			return nil, ErrEmptyChannel
		}
	}
	// Default: still time left to wait for results
	select {
	case resp := <-in:
		return resp, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (j *job) remainingTasks() []*proto.SMCCmd {
	if int(j.progress) >= len(j.instr.Tasks) {
		return []*proto.SMCCmd{}
	}
	return j.instr.Tasks[j.progress:]
}

func (j *job) incProgress() {
	j.progress++
}

// API facing job originator for interaction

func (j *job) sendFeedback(resp *proto.CmdResult) {
	j.feedback <- PeerResult{Progress: j.progress, Response: resp}
}

func (j *job) abort(e *PeerError) {
	j.mu.Lock()
	j.lastErr = e
	j.mu.Unlock()
	// Notify client (job originator) about error
	close(j.feedback)

	j.closeAllChats()
}
