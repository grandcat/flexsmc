package wiring

import (
	"context"
	"fmt"
	"log"

	"github.com/grandcat/flexsmc/directory"
	proto "github.com/grandcat/flexsmc/proto"
)

type TaskWatcher interface {
	Abort()
	Result() <-chan interface{}
}

type task struct {
	feedback chan interface{}
	// Task context and instructions
	ctx     context.Context
	targets []*directory.PeerInfo
	smcCmd  *proto.SMCCmd
}

func newTask(ctx context.Context, targets []*directory.PeerInfo, cmd *proto.SMCCmd) *task {
	return &task{
		feedback: make(chan interface{}),
		ctx:      ctx,
		targets:  targets,
		smcCmd:   cmd,
	}
}

func (t *task) Abort() {
	// TODO
}

func (t *task) Result() <-chan interface{} {
	return t.feedback
}

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

func (pc *PeerConnection) SendTask(ctx context.Context, dest []*directory.PeerInfo, cmd *proto.SMCCmd) (TaskWatcher, error) {
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

func (pc *PeerConnection) taskWorker() {
	for t := range pc.tasks {
		fmt.Println("Incomming task:", t)

		chats := make([]directory.ChatWithPeer, 0, len(t.targets))
		for _, p := range t.targets {
			// Initiate communication channel to each peer.
			// If it takes too much time until the corresponding peer reacts on our
			// talk request, we need to inform the originator of our task.
			peerChat, err := p.RequestChat(context.Background())
			if err != nil {
				log.Printf("[%s] Talk request not handled fast enough. Aborting.", p.ID)
				continue
			}
			chats = append(chats, peerChat)

			peerChat.Instruct() <- t.smcCmd
			// XXX: fetch response
			response := <-peerChat.GetFeedback()
			log.Println(">>Response from peer:", response)
		}

		// Now check all results

	}
}
