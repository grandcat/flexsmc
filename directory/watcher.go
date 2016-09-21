package directory

import (
	"log"
	"sync"

	auth "github.com/grandcat/srpc/authentication"
)

type peerWatcher struct {
	peersOn map[auth.PeerID]*PeerInfo
	mu      sync.RWMutex // Needed if GW wants a list of available peers

	notifies chan stateChange
}

type stateChange struct {
	p     *PeerInfo
	avail PeerAvailability
}

func newPeerWatcher() *peerWatcher {
	w := &peerWatcher{
		peersOn:  make(map[auth.PeerID]*PeerInfo),
		notifies: make(chan stateChange), // cap = 0, blocking
	}

	go w.watch()

	return w
}

func (w *peerWatcher) watch() {
	log.Println("Starting peerWatcher")

	for n := range w.notifies {
		// Update our view on available nodes
		if n.avail == 0 {
			// Peer went offline (temporary or forever)
			w.mu.Lock()
			delete(w.peersOn, n.p.ID)
			w.mu.Unlock()

			log.Printf("[%s] Offline or unreachable", n.p.ID)
		} else {
			// Peer went online
			w.mu.Lock()
			w.peersOn[n.p.ID] = n.p
			w.mu.Unlock()

			log.Printf("[%s] Online", n.p.ID)
		}
		// List peers currently available
		log.Println("Online peers:", w.peersOn)
	}
}

func (w *peerWatcher) Notifications() chan<- stateChange {
	return w.notifies
}
