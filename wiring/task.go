package wiring

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/grandcat/flexsmc/directory"
	proto "github.com/grandcat/flexsmc/proto"
	auth "github.com/grandcat/srpc/authentication"
)

type TaskWatcher interface {
	Result() <-chan *proto.CmdResult
	// Err is non-nil if a critical error occurred during operation.
	// It should be called first when Result() chan was called from our side.
	Err() *PeerError
}

type task struct {
	feedback chan *proto.CmdResult
	lastErr  *PeerError
	mu       sync.Mutex
	// Static task context and instructions
	ctx     context.Context
	targets []*directory.PeerInfo
	smcCmd  *proto.SMCCmd
	// Context for worker processing this task
	chats map[auth.PeerID]directory.ChatWithPeer
}

func newTask(ctx context.Context, targets []*directory.PeerInfo, cmd *proto.SMCCmd) *task {
	return &task{
		feedback: make(chan *proto.CmdResult),
		ctx:      ctx,
		targets:  targets,
		smcCmd:   cmd,
	}
}

func (t *task) Result() <-chan *proto.CmdResult {
	return t.feedback
}

func (t *task) Err() *PeerError {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastErr
}

// openPeerChats initiates a bi-directional communication channel to each peer.
// If it takes too much time until the corresponding peer reacts on our
// talk request, we need to inform the originator of this task.
func (t *task) openPeerChats(ctx context.Context) *PeerError {
	t.chats = make(map[auth.PeerID]directory.ChatWithPeer, len(t.targets))
	var errPeers []*directory.PeerInfo

	for _, p := range t.targets {
		pch, err := p.RequestChat(ctx)
		if err != nil {
			errPeers = append(errPeers, p)
			log.Printf("[%s] Talk request not handled fast enough. Aborting.", p.ID)
			continue
		}
		t.chats[p.ID] = pch
	}

	if len(errPeers) > 0 {
		return NewPeerErr(ctx.Err(), errPeers)
	}
	return nil
}

// removePeerChat deletes entry from map of active chats.
func (t *task) removePeerChats(peers []*directory.PeerInfo) {
	for _, p := range peers {
		delete(t.chats, p.ID)
	}
}

func (t *task) closeAllChats() {
	for _, ch := range t.chats {
		ch.Close()
	}
	t.chats = nil
}

var errCtxOrStreamFailure = errors.New("ctx timeout or stream failure")

func (t *task) queryTargetsSync(ctx context.Context, cmd *proto.SMCCmd) ([]*proto.CmdResult, *PeerError) {
	// First, disseminate the task to all peers.
	// Then, collect all results, but expect the results to be there until timeout occurs.

	// Send
	for _, pch := range t.chats {
		pch.Instruct() <- cmd
	}
	// Receive
	// Each peer delivers its response independently from each other. If one peer blocks,
	// there is still the result of all other peers after the timeout occurs.
	var errPeers []*directory.PeerInfo
	resps := make([]*proto.CmdResult, 0, len(t.chats))
	for _, pch := range t.chats {
		resp, err := pullRespUntilDone(ctx, pch.GetFeedback())
		switch {
		case err != nil:
			fallthrough
		case resp.Status >= proto.CmdResult_STREAM_ERR:
			errPeers = append(errPeers, pch.Peer())
			log.Printf("[%s] Task communication failed: %v", pch.Peer().ID, err)
			pch.Close()
			continue
		}
		resps = append(resps, resp)
		log.Printf(">>[%s] Response from peer: %v", pch.Peer().ID, resp)
	}

	var err *PeerError
	if len(errPeers) > 0 {
		t.removePeerChats(errPeers)
		err = NewPeerErr(errCtxOrStreamFailure, errPeers)
	}
	return resps, err
}

func (t *task) abort(e *PeerError) {
	t.mu.Lock()
	t.lastErr = e
	t.mu.Unlock()
	// Notify client (task originator) about error
	close(t.feedback)

	t.closeAllChats()
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
