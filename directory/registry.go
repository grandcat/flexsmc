package directory

import (
	"errors"
	"log"
	"sync"

	auth "github.com/grandcat/srpc/authentication"
)

var (
	ErrNotRegistered = errors.New("peer ID not registered")
)

type Registry struct {
	peers map[auth.PeerID]*PeerInfo
	mu    sync.RWMutex

	watcher *peerWatcher
}

func NewRegistry() *Registry {
	return &Registry{
		peers:   make(map[auth.PeerID]*PeerInfo),
		watcher: newPeerWatcher(),
	}
}

func (d *Registry) Add(p *PeerInfo) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.peers[p.ID]; ok {
		// A peer with this ID is already registered
		log.Printf("Peer %s already exists.", p.ID)
		return
	}

	d.peers[p.ID] = p
}

func (d *Registry) GetOrCreate(id auth.PeerID) *PeerInfo {
	d.mu.RLock()
	p, ok := d.peers[id]
	d.mu.RUnlock()
	if !ok {
		p = &PeerInfo{
			ID:            id,
			stateNotifier: d.watcher.Notifications(),
		}
		d.Add(p)
	}
	return p
}

func (d *Registry) Get(id auth.PeerID) (*PeerInfo, error) {
	d.mu.RLock()
	p, ok := d.peers[id]
	d.mu.RUnlock()
	if !ok {
		return nil, ErrNotRegistered
	}
	return p, nil
}

// BroadcastMsg sends a message to the listed receivers.
// Peers that are currently not reachable, are ignored. No recovery.
// func (d *Registry) BroadcastMsg(rcvs []auth.PeerID, m interface{}) {
// 	// d.mu.RLock()
// 	// defer d.mu.RUnlock()

// 	// Intersect with peers online before transmission
// 	for _, pID := range rcvs {
// 		if p, ok := d.readyPeers[pID]; ok {
// 			p.tx <- m
// 			log.Printf(">> Msg for %s: %v", pID, m)
// 		}
// 	}
// }
