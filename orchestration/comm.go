package orchestration

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/grandcat/flexsmc/directory"
	proto "github.com/grandcat/flexsmc/proto"
)

type PeerNetwork struct {
	reg  *directory.Registry
	jobs chan *job
}

func NewPeerNetwork(r *directory.Registry) *PeerNetwork {
	pc := &PeerNetwork{
		reg:  r,
		jobs: make(chan *job, 4),
	}
	// Spawn some workers for communication with peers
	go pc.jobWorker()
	go pc.jobWorker()

	log.Println("Starting workers for managing peer connections")

	return pc
}

func (pc *PeerNetwork) SubmitJob(ctx context.Context, dest []*directory.PeerInfo, cmd *proto.SMCCmd) (JobWatcher, error) {
	t := newJob(ctx, dest, cmd)
	// Enqueue for worker pool
	select {
	case pc.jobs <- t:
		// Everything fine. Soon, an available worker should start processing the task.
	default:
		return nil, fmt.Errorf("too many tasks running")
	}

	return t, nil
}

func (pc *PeerNetwork) RescheduleOpenTask() {
	// TODO:
	// Reuse an existing tasks with opened chats so we do not need to reopen them.
	// This safes some time and resources.
}

func (pc *PeerNetwork) jobWorker() {
	for t := range pc.jobs {
		fmt.Println("Task starts:", t)
		processJob(t)
		fmt.Println("Task ends:", t)
	}
}

func processJob(t *job) {
	prepCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	err := t.openPeerChats(prepCtx)
	if err != nil {
		// Minimum 1 tchat failed. Abort early so GW can schedule the same task for a subset
		// of the current target peers if applicable.
		t.abort(err)
		return
	}
	// Prepare phase with task
	_, err = t.queryTargetsSync(prepCtx, t.smcCmd)
	if err != nil {
		t.abort(err)
		return
	}
	// Trigger session and wait for final results
	sp := &proto.SMCCmd{
		State:   proto.SMCCmd_SESSION,
		Payload: &proto.SMCCmd_Session{&proto.SessionPhase{}},
	}
	results, err := t.queryTargetsSync(t.ctx, sp)
	// Send back results if there are any
	// XXX: replace with direct routing of results to the client
	for _, r := range results {
		t.feedback <- r
	}
	if err != nil {
		t.abort(err)
		return
	}
	// Notify client that we are done here.
	close(t.feedback)

	// No more instructions from our side for the group of peers.
	t.closeAllChats()
}
