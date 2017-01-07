package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/grandcat/flexsmc/directory"
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

	glog.V(1).Infoln("Starting workers for managing peer connections")

	return pc
}

func (pc *PeerNetwork) SubmitJob(ctx context.Context, description JobInstruction, opts ...JobOption) (JobWatcher, error) {
	j := newJob(ctx, description, opts)
	// Enqueue for worker pool
	select {
	case pc.jobs <- j:
		// Everything fine. Soon, an available worker should start processing the task.
	case <-ctx.Done():
		return nil, fmt.Errorf("could not submit job: %s", ctx.Err().Error())
	}

	return j, nil
}

func (pc *PeerNetwork) RescheduleOpenJob(ctx context.Context, haltedJob JobWatcher, instruction JobInstruction, opts ...JobOption) error {
	j, ok := haltedJob.(*job)
	if !ok {
		panic("Provided JobWatcher not compatible to internal job struct.")
	}

	err := j.recycleJob(ctx, instruction, opts)
	if err != nil {
		return err
	}

	select {
	case pc.jobs <- j:
		// Schedule new task. Everything is fine.
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (pc *PeerNetwork) jobWorker() {
	for t := range pc.jobs {
		glog.V(1).Infoln("Task starts:", t)
		processJob(t)
		glog.V(1).Infoln("Task ends:", t)
	}
}

func processJob(j *job) {
	if ctxErr := j.ctx.Err(); ctxErr != nil {
		// Job already expired before we could work on it.
		// Notify callee before even trying to contact our peers.
		j.haltOrAbort(newPeerErr(ctxErr, Aborted, -1, nil))
		return
	}
	// XXX: use default job context for this as well?
	prepCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	err := j.openPeerChats(prepCtx)
	if err != nil {
		// Minimum 1 chat failed. Abort early so GW can schedule the same task for a subset
		// of the current target peers if applicable.
		j.haltOrAbort(err)
		return
	}
	// Work through all phases
	cmds := j.remainingTasks()
	for _, t := range cmds {
		results, err := j.queryTargetsSync(j.ctx, t)
		// Send back results if there are any
		// XXX: replace with direct routing of results to the client
		for _, r := range results {
			j.sendFeedback(r)
		}
		if err != nil {
			j.haltOrAbort(err)
			return
		}

		j.incProgress()
	}
	// Notify client that we are done here.
	close(j.feedback)
	// No more instructions from our side for the group of peers.
	j.closeAllChats()
}
