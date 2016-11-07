package worker

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/grandcat/flexsmc/directory"
	pbJob "github.com/grandcat/flexsmc/proto/job"
	auth "github.com/grandcat/srpc/authentication"
)

var (
	ErrNewParticipants   = errors.New("cannot reuse job for new participants")
	ErrIncompatibleTasks = errors.New("tasks of old and new job are incompatible")
	ErrNotHalted         = errors.New("job is not halted")
)

type JobInstruction struct {
	Tasks        []*pbJob.SMCCmd
	Participants map[directory.ChannelID]*directory.PeerInfo
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
	// Abort tears down all connected chats. It only works if the job is
	// currently halted.
	Abort() error
	// Err is non-nil if a critical error occurred during operation.
	// It should be called first when Result() chan was closed from our side.
	Err() *PeerError
}

type JobPhase int32 //< slightly coupled to pbJob.SMCCmd_Phase

type PeerResult struct {
	Progress JobPhase
	Response *pbJob.CmdResult
}

type job struct {
	feedback chan PeerResult
	lastErr  *PeerError
	mu       sync.Mutex
	// Instruction with job context, target peers and their task to do
	ctx   context.Context
	instr JobInstruction
	// Context for worker processing this job
	progress JobPhase
	chats    map[auth.PeerID]directory.ChatWithPeer
}

func newJob(ctx context.Context, instruction JobInstruction) *job {
	return &job{
		feedback: make(chan PeerResult),
		ctx:      ctx,
		instr:    instruction,
	}
}

func (j *job) recycleJob(ctx context.Context, newInstr JobInstruction) error {
	// Jobs still compatible?
	// XXX: do more in-depth incompatibility check
	if len(newInstr.Tasks) < len(j.instr.Tasks) {
		return ErrIncompatibleTasks
	}
	// All participants must already be connected. New ones are not accepted as
	// their progress will not be in sync to the rest.
	for cid, p := range newInstr.Participants {
		pch, exists := j.chats[p.ID]
		if !exists {
			return ErrNewParticipants
		}
		// Set new channelID as it might have changed.
		pch.UpdateMetadata(cid)
	}
	// All required chats are there, but there can be more than that. Clean up.
	if true || len(newInstr.Participants) < len(j.chats) {
		j.closeUnusedChats()
	}

	j.mu.Lock()
	j.ctx = ctx
	j.feedback = make(chan PeerResult)
	j.lastErr = nil
	j.instr = newInstr
	j.mu.Unlock()

	return nil
}

func (j *job) Job() JobInstruction {
	return j.instr
}

func (j *job) Result() <-chan PeerResult {
	return j.feedback
}

func (j *job) Abort() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	// Job is still running. Do not allow abort here for now.
	if j.lastErr == nil {
		return ErrNotHalted
	}

	if j.lastErr.Status == Halted {
		j.closeAllChats()
		j.lastErr = nil
	}
	log.Printf("Job aborted successfully.")

	return nil
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

	for cid, p := range j.instr.Participants {
		if _, exists := j.chats[p.ID]; exists {
			continue
		}
		pch, err := p.RequestChat(ctx, cid)
		if err != nil {
			errPeers = append(errPeers, p)
			log.Printf("[%s] Talk request not handled fast enough. Aborting.", p.ID)
			continue
		}
		j.chats[p.ID] = pch
	}

	if len(errPeers) > 0 {
		// TODO: set status depending on what the requestor wants to handle (-> handleErrFlags)
		return NewPeerErr(ctx.Err(), Aborted, j.progress, errPeers)
	}
	return nil
}

// revokePeerChats tears down and delete the chats to these peers.
// It is essential to do so that a resubmitted job has a new chance to
// initiate a new chat to a previously faulty peer.
func (j *job) revokePeerChats(peers []*directory.PeerInfo) {
	for _, p := range peers {
		j.revokePeerChat(p)
	}
}

func (j *job) revokePeerChat(p *directory.PeerInfo) {
	// Note: add mutex if multiple goroutines alter this structure.
	if pch, ok := j.chats[p.ID]; ok {
		pch.Close()
		delete(j.chats, p.ID)
	}
}

func (j *job) closeUnusedChats() {
	for _, pch := range j.chats {
		connPeer := pch.Peer()
		needed := false
	inner:
		for _, reqPeer := range j.instr.Participants {
			if connPeer == reqPeer {
				needed = true
				break inner
			}
		}
		if !needed {
			j.revokePeerChat(connPeer)
			log.Printf("Remove unused peer chat for %v", connPeer.ID)
		}
	}
}

func (j *job) closeAllChats() {
	for _, pch := range j.chats {
		pch.Close()
	}
	j.chats = nil
}

var ErrCtxOrStreamFailure = errors.New("ctx timeout or stream failure")

func (j *job) queryTargetsSync(ctx context.Context, cmd *pbJob.SMCCmd) ([]*pbJob.CmdResult, *PeerError) {
	// First, disseminate the job to all peers.
	// Then, collect all results, but expect the results to be there until timeout occurs.
	for _, pch := range j.chats {
		pch.InstructSafe(cmd)
	}
	// Receive
	// Each peer delivers its response independently from each other. If one peer blocks,
	// there is still the result of all other peers after the timeout occurs.
	handleErrFlags := pbJob.CmdResult_ERR_CLASS_NORM | pbJob.CmdResult_ERR_CLASS_COMM

	var accumulatedErrFlags pbJob.CmdResult_Status
	var errPeers []*directory.PeerInfo
	resps := make([]*pbJob.CmdResult, 0, len(j.chats))
	for _, pch := range j.chats {
		resp, commErr := pullRespUntilDone(ctx, pch.GetFeedback())

		var errFlag pbJob.CmdResult_Status
		switch {
		case commErr != nil:
			errFlag = pbJob.CmdResult_STREAM_ERR
		case (resp.Status & pbJob.CmdResult_ALL_ERROR_CLASSES) > 0:
			errFlag = resp.Status & pbJob.CmdResult_ALL_ERROR_CLASSES
		}
		if (errFlag & pbJob.CmdResult_ALL_ERROR_CLASSES) > 0 {
			accumulatedErrFlags |= errFlag
			errPeers = append(errPeers, pch.Peer())
			log.Printf("[%s] peer job failed: %v, Reason: %s",
				pch.Peer().ID, commErr, resp.Status.String())

			// Special case: communication errors
			// Remove affected peers directly as their communication channel died anyway
			// and might cause problems in future when a job is reassigned.
			if (errFlag & pbJob.CmdResult_ERR_CLASS_COMM) > 0 {
				j.revokePeerChat(pch.Peer())
				log.Printf("[%s] kicking peer. Comm chan died.", pch.Peer().ID)
			}
			continue
		}

		resps = append(resps, resp)
		log.Printf(">>[%s] Response from peer: %v", pch.Peer().ID, resp)
	}

	var err *PeerError
	status := Halted
	switch {
	// If any error is unhandled by the originator of this job, we cannot proceed.
	// Mark job as aborted.
	case (handleErrFlags & accumulatedErrFlags) != accumulatedErrFlags:
		j.revokePeerChats(errPeers)
		status = Aborted
		fallthrough
	case len(errPeers) > 0:
		err = NewPeerErr(ErrCtxOrStreamFailure, status, j.progress, errPeers)
	}

	return resps, err
}

var ErrEmptyChannel = errors.New("input channel is empty")
var ErrChanClosed = errors.New("peer chan closed")

func pullRespUntilDone(ctx context.Context, in <-chan *pbJob.CmdResult) (*pbJob.CmdResult, error) {
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
	case resp, ok := <-in:
		if !ok {
			return resp, ErrChanClosed
		}
		return resp, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (j *job) remainingTasks() []*pbJob.SMCCmd {
	if int(j.progress) >= len(j.instr.Tasks) {
		return []*pbJob.SMCCmd{}
	}
	return j.instr.Tasks[j.progress:]
}

func (j *job) incProgress() {
	j.progress++
}

// API facing job originator for interaction

func (j *job) sendFeedback(resp *pbJob.CmdResult) {
	j.feedback <- PeerResult{Progress: j.progress, Response: resp}
}

func (j *job) haltOrAbort(e *PeerError) {
	j.mu.Lock()
	j.lastErr = e
	j.mu.Unlock()
	// Notify client (job originator) about error
	close(j.feedback)

	if e.Status == Aborted {
		j.closeAllChats()
		log.Printf("Job aborted. Closing all comm channels.")
	}
}
