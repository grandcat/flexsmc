package wiring

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/grandcat/flexsmc/directory"
	proto "github.com/grandcat/flexsmc/proto"
)

type PeerConnection struct {
	reg   *directory.Registry
	tasks chan *task
}

func NewPeerConnection(r *directory.Registry) *PeerConnection {
	pc := &PeerConnection{
		reg:   r,
		tasks: make(chan *task, 4),
	}
	// Spawn some workers for communication with peers
	go pc.taskWorker()
	go pc.taskWorker()

	log.Println("Starting workers for managing peer connections")

	return pc
}

func (pc *PeerConnection) SubmitTask(ctx context.Context, dest []*directory.PeerInfo, cmd *proto.SMCCmd) (TaskWatcher, error) {
	t := newTask(ctx, dest, cmd)
	// Enqueue for worker pool
	select {
	case pc.tasks <- t:
		// Everything fine. Soon, an available worker should start processing the task.
	default:
		return nil, fmt.Errorf("too many tasks running")
	}

	return t, nil
}

func (pc *PeerConnection) RescheduleOpenTask() {
	// TODO:
	// Reuse an existing tasks with opened chats so we do not need to reopen them.
	// This safes some time and resources.
}

func (pc *PeerConnection) taskWorker() {
	for t := range pc.tasks {
		fmt.Println("Task starts:", t)
		processTask(t)
		fmt.Println("Task ends:", t)
	}
}

func processTask(t *task) {
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
