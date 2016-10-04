package directory

import (
	"log"
	"sync"

	auth "github.com/grandcat/srpc/authentication"
)

type peerStatus struct {
	peer       *PeerInfo
	aliveConns uint
}

type peerWatcher struct {
	peersOn map[auth.PeerID]peerStatus
	mu      sync.RWMutex // Needed if GW wants a list of available peers

	notifies chan stateChange
}

type stateChange struct {
	p     *PeerInfo
	avail PeerAvailability
}

func newPeerWatcher() *peerWatcher {
	w := &peerWatcher{
		peersOn:  make(map[auth.PeerID]peerStatus),
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
			w.delOrDec(n.p)

		} else {
			// Peer went online or added another simultaneous connection.
			w.addOrInc(n.p)
		}
		// List peers currently available
		log.Println("Online peers:", w.peersOn)
	}
}

func (w *peerWatcher) addOrInc(peer *PeerInfo) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if s, ok := w.peersOn[peer.ID]; ok {
		s.aliveConns++
		w.peersOn[peer.ID] = s
		log.Printf("[%s] %d active conns", peer.ID, s.aliveConns)

	} else {
		w.peersOn[peer.ID] = peerStatus{peer: peer, aliveConns: 1}
		log.Printf("[%s] Online", peer.ID)
	}
}

func (w *peerWatcher) delOrDec(peer *PeerInfo) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if s, ok := w.peersOn[peer.ID]; ok {
		if s.aliveConns == 1 {
			delete(w.peersOn, peer.ID)
			log.Printf("[%s] Offline or unreachable", peer.ID)

		} else {
			s.aliveConns--
			w.peersOn[peer.ID] = s
			log.Printf("[%s] %d active conns", peer.ID, s.aliveConns)
		}
	}
}

func (w *peerWatcher) Notifications() chan<- stateChange {
	return w.notifies
}

func (w *peerWatcher) AvailablePeers() []*PeerInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()

	nodes := make([]*PeerInfo, 0, len(w.peersOn))
	for _, ps := range w.peersOn {
		nodes = append(nodes, ps.peer)
	}

	return nodes
}
