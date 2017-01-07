package directory

import (
	"sync"

	"bytes"

	"github.com/golang/glog"
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
		peersOn: make(map[auth.PeerID]peerStatus),
		// Choose big enough, so it does not become the bottleneck for
		// new communication channels attaching to the GW.
		notifies: make(chan stateChange, 32),
	}

	go w.watch()

	return w
}

func (w *peerWatcher) watch() {
	glog.V(1).Infoln("Starting peerWatcher")

	for n := range w.notifies {
		// Update our view on available nodes
		var change int
		if n.avail == 0 {
			// Peer went offline (temporary or forever)
			change = w.delOrDec(n.p)

		} else {
			// Peer went online or added another simultaneous connection.
			change = w.addOrInc(n.p)
		}
		// List peers currently online if their presence changed.
		if change != 0 {
			glog.V(1).Infof("Online peers %+d:\n%s", change, w.printPeersOnline())
		}
	}
}

func (w *peerWatcher) addOrInc(peer *PeerInfo) (globChange int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if s, ok := w.peersOn[peer.ID]; ok {
		s.aliveConns++
		w.peersOn[peer.ID] = s
		glog.V(4).Infof("[%s] %d active conns", peer.ID, s.aliveConns)

	} else {
		w.peersOn[peer.ID] = peerStatus{peer: peer, aliveConns: 1}
		globChange = 1
		glog.V(4).Infof("[%s] Online", peer.ID)
	}
	return
}

func (w *peerWatcher) delOrDec(peer *PeerInfo) (globChange int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if s, ok := w.peersOn[peer.ID]; ok {
		if s.aliveConns == 1 {
			delete(w.peersOn, peer.ID)
			globChange = -1
			glog.V(4).Infof("[%s] Offline or unreachable", peer.ID)

		} else {
			s.aliveConns--
			w.peersOn[peer.ID] = s
			glog.V(4).Infof("[%s] %d active conns", peer.ID, s.aliveConns)
		}
	}
	return
}

func (w *peerWatcher) Notifications() chan<- stateChange {
	return w.notifies
}

func (w *peerWatcher) AvailablePeers() map[auth.PeerID]struct{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	nodes := make(map[auth.PeerID]struct{}, len(w.peersOn))
	for _, ps := range w.peersOn {
		nodes[ps.peer.ID] = struct{}{}
	}

	return nodes
}

func (w *peerWatcher) printPeersOnline() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var out bytes.Buffer
	for pid := range w.peersOn {
		out.WriteString(" + [")
		out.WriteString(string(pid))
		out.WriteString("]\n")
	}
	return out.String()
}
