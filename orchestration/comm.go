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

func (pc *PeerNetwork) SubmitJob(ctx context.Context, dest []*directory.PeerInfo, tasks []*proto.SMCCmd) (JobWatcher, error) {
	instr := JobInstruction{
		Context: ctx,
		Targets: dest,
		Tasks:   tasks,
	}
	j := newJob(instr)
	// Enqueue for worker pool
	select {
	case pc.jobs <- j:
		// Everything fine. Soon, an available worker should start processing the task.
	default:
		return nil, fmt.Errorf("too many tasks running")
	}

	return j, nil
}

func (pc *PeerNetwork) RescheduleOpenTask() {
	// TODO:
	// Reuse an existing tasks with opened chats so we do not need to reopen them.
	// This safes some time and resources.
	// Currently the assumption is that peers might only be removed. Adding new peers
	// might result in undefined behavior as they missed phases previously sent
	// (if some progress was made).
}

func (pc *PeerNetwork) jobWorker() {
	for t := range pc.jobs {
		fmt.Println("Task starts:", t)
		processJob(t)
		fmt.Println("Task ends:", t)
	}
}

func processJob(j *job) {
	prepCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	err := j.openPeerChats(prepCtx)
	if err != nil {
		// Minimum 1 chat failed. Abort early so GW can schedule the same task for a subset
		// of the current target peers if applicable.
		j.abort(err)
		return
	}
	// Work through all phases
	cmds := j.remainingTasks()
	for _, t := range cmds {
		results, err := j.queryTargetsSync(j.instr.Context, t)
		// Send back results if there are any
		// XXX: replace with direct routing of results to the client
		for _, r := range results {
			j.sendFeedback(r)
		}
		if err != nil {
			j.abort(err)
			return
		}

		j.incProgress()
	}
	// Notify client that we are done here.
	close(j.feedback)

	// No more instructions from our side for the group of peers.
	j.closeAllChats()
}
