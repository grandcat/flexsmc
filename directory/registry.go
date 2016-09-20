package directory

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"

	proto "github.com/grandcat/flexsmc/proto"
	auth "github.com/grandcat/srpc/authentication"
)

var (
	ErrNotRegistered = errors.New("peer ID not registered")
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

	// Comm interface for messages from GW -> peer
	tx chan interface{}
}

type Registry struct {
	peers map[auth.PeerID]*PeerInfo
	mu    sync.RWMutex
	// Peers' feedback: peers -> GW's registry
	rx <-chan interface{}
}

func NewRegistry() *Registry {
	return &Registry{
		peers: make(map[auth.PeerID]*PeerInfo),
		rx:    make(chan interface{}, 32),
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

func (d *Registry) Touch(id auth.PeerID, addr net.Addr, smcPort uint16) {
	// TODO: check if gw accepts peer for SMC actions
	d.mu.Lock()
	defer d.mu.Unlock()

	p, ok := d.peers[id]
	if !ok {
		d.peers[id] = &PeerInfo{}
		p = d.peers[id]
	}
	// Assume TCP endpoint and extract peer IP address.
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		p.Addr = net.TCPAddr{IP: tcpAddr.IP, Port: int(smcPort)}
	}
	p.lastPing = time.Now()
}

// NoActivity means that a peer was inactive for a very long time to
// disregard it for the time being.
// The duration is around 42 years.
const NoActivity time.Duration = time.Hour * 24 * 365 * 42

func (d *Registry) LastActivity(id auth.PeerID) time.Duration {
	d.mu.RLock()
	defer d.mu.RUnlock()

	p, ok := d.peers[id]
	if !ok {
		return NoActivity
	}
	if p.lastPing.IsZero() {
		return NoActivity
	}
	return time.Since(p.lastPing)
}

func (d *Registry) SubscribeCmdChan(id auth.PeerID) (<-chan interface{}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	p, ok := d.peers[id]
	if !ok {
		return nil, ErrNotRegistered
	}
	// We assume that the previous chan was closed via UnsubscribeCmdChan().
	p.tx = make(chan interface{}, 4)
	log.Println("Create new comm chan directed to peer", id)

	// XXX: test sending some messages
	go func() {
		time.Sleep(time.Second * 2)
		for i := 2; i >= 0; i-- {
			prep := proto.SMCCmd_Prepare{&proto.Prepare{
				Participants: []*proto.Prepare_Participant{&proto.Prepare_Participant{Addr: "myAddr", Identity: "ident"}},
			}}
			p.tx <- proto.SMCCmd{Type: proto.SMCCmd_Type(i), Payload: &prep}
		}
	}()

	return p.tx, nil
}

func (d *Registry) UnsubscribeCmdChan(id auth.PeerID) {
	d.mu.Lock()
	defer d.mu.Unlock()

	p, ok := d.peers[id]
	if !ok {
		return
	}

	close(p.tx) // NOTE: panics if chan is closed already!
	log.Println("Close comm chan to peer", id)
}
