package directory

import (
	"errors"
	"sync"

	"github.com/grandcat/flexsmc/logs"
	auth "github.com/grandcat/srpc/authentication"
)

var (
	ErrNotRegistered = errors.New("peer ID not registered")
)

type Registry struct {
	peers map[auth.PeerID]*PeerInfo
	mu    sync.RWMutex

	Watcher *peerWatcher
}

func NewRegistry() *Registry {
	return &Registry{
		peers:   make(map[auth.PeerID]*PeerInfo),
		Watcher: newPeerWatcher(),
	}
}

func (d *Registry) Add(p *PeerInfo) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.peers[p.ID]; ok {
		// A peer with this ID is already registered
		logs.Warningf("Peer %s already exists.", p.ID)
		return
	}

	d.peers[p.ID] = p
}

func (d *Registry) GetOrCreate(id auth.PeerID) *PeerInfo {
	d.mu.RLock()
	p, ok := d.peers[id]
	d.mu.RUnlock()
	if !ok {
		p = newPeerInfo(id, d.Watcher.Notifications())
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

func (d *Registry) GetAll() []*PeerInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var peers []*PeerInfo
	for _, p := range d.peers {
		peers = append(peers, p)
	}

	return peers
}
