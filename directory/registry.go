package directory

import (
	"log"
	"net"
	"sync"
	"time"

	auth "github.com/grandcat/srpc/authentication"
)

type PeerRole uint8

const (
	// Peer is a regular workerpeer.
	Peer PeerRole = iota
	// GEP is a Gateway Eligible Peer.
	// Normally, it behaves the same way as a regular peer. As it has
	// more computational resources, it is a possible candidate for a
	// GW position.
	GEP
	// GW is a Gateway role.
	// In each peer network, there should be exactly one peer
	// representing this role.
	GW
)

type PeerInfo struct {
	// ID is typically the CommonName of a peer node.
	// The same identifies the peerID for the cert manager.
	ID auth.PeerID
	// Addr is the IP and port providing the SMC endpoint.
	// The IP is derived from the connection source address on pinging.
	Addr     net.TCPAddr
	lastPing time.Time

	Version int32
	Role    PeerRole

	// Capabilities
	// TODO: replace with proto data structure
	Caps string
	// TODO: metrics about errors, peers' reliability, ...

	// TODO: communication channel?
}

type Registry struct {
	peers map[auth.PeerID]*PeerInfo
	mu    sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		peers: make(map[auth.PeerID]*PeerInfo),
	}
}

func (d *Registry) Add(p PeerInfo) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.peers[p.ID]; ok {
		// A peer with this ID is already registered
		log.Printf("Peer %s already exists.", p.ID)
		return
	}

	d.peers[p.ID] = &p
}

func (d *Registry) Touch(id auth.PeerID, addr net.Addr, smcPort uint) {
	// TODO: check if gw accepts peer for SMC actions
	d.mu.Lock()
	defer d.mu.Unlock()

	p, ok := d.peers[id]
	if !ok {
		d.peers[id] = &PeerInfo{}
		p = d.peers[id]
	}
	// Assume TCP endpoint and extract peer IP address.
	//
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		p.Addr = net.TCPAddr{IP: tcpAddr.IP, Port: int(smcPort)}
	}
	p.lastPing = time.Now()
}

func (d *Registry) LastActivity(id auth.PeerID) time.Duration {
	d.mu.RLock()
	defer d.mu.RUnlock()

	p, ok := d.peers[id]
	if !ok {
		return -1
	}
	if p.lastPing.IsZero() {
		return -1
	}
	return time.Since(p.lastPing)
}
