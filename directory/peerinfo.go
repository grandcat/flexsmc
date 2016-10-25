package directory

import (
	"context"
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
	// more computational resources, it is a potential candidate to take
	// over the GW role if the current GW fails.
	GEP
	// GW is a Gateway role.
	// In each peer network, there should be exactly one peer
	// representing this role.
	GW
)

// PeerAvailability states if a peer is ready for operation (>0) or offline (0)
type PeerAvailability uint8

type PeerInfo struct {
	mu sync.Mutex
	// ID is typically the CommonName of a peer node.
	// The same identifies the peerID for the cert manager.
	ID auth.PeerID
	// Addr is the IP and port providing the SMC endpoint.
	// The IP is derived from the connection source address on pinging.
	// lastPing states the last activity a peer connected to the gateway.
	// Note: both Addr and lastPing receive regular updates.
	Addr     net.TCPAddr
	lastPing time.Time

	Version int32
	Role    PeerRole

	// Capabilities
	Caps string
	// TODO: metrics about errors, peers' reliability, ...

	// Bi-directional communication requests
	requestedSessions chan ChatWithGateway
	stateNotifier     chan<- stateChange //< Needs external initialization
}

func newPeerInfo(id auth.PeerID, doNotify chan<- stateChange) *PeerInfo {
	return &PeerInfo{
		ID:                id,
		requestedSessions: make(chan ChatWithGateway),
		stateNotifier:     doNotify,
	}
}

func (pi *PeerInfo) Touch(addr net.Addr) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	// Assume TCP endpoint and extract peer IP address.
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		pi.Addr = net.TCPAddr{IP: tcpAddr.IP}
	}
	pi.lastPing = time.Now()
}

const (
	// NoActivity means that a peer was inactive for a very long time to
	// disregard it for the time being.
	// The duration is around 42 years.
	NoActivity time.Duration = time.Hour * 24 * 365 * 42
	// MaxActivityGap is the maximum allowed period in seconds a peer can
	// be passive without sending a keep-alive message.
	MaxActivityGap = 20
)

func (pi *PeerInfo) LastActivity() time.Duration {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	if pi.lastPing.IsZero() {
		return NoActivity
	}
	// Think about: use this trigger to clean up unused channels
	return time.Since(pi.lastPing)
}

func (pi *PeerInfo) SubscribeCmdChan() <-chan ChatWithGateway {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	// We assume that the previous chan was closed via UnsubscribeCmdChan().
	log.Printf("PeerInfo: [%s] SubscribeCmdChan ", pi.ID)
	// Add peer to pool of available peers
	pi.stateNotifier <- stateChange{avail: 2, p: pi}

	return pi.requestedSessions
}

func (pi *PeerInfo) UnsubscribeCmdChan() {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	// Remove peer from directory of available peers
	// Notify peer watcher about lost communication path
	pi.stateNotifier <- stateChange{avail: 0, p: pi}

	// Not necessary to close the channel if this signal is unused.
	// See https://groups.google.com/forum/#!msg/golang-nuts/pZwdYRGxCIk/qpbHxRRPJdUJ .
	// close(pi.tx) // NOTE: panics if chan is closed already!
	log.Printf("PeerInfo: [%s] UnsubscribeCmdChan", pi.ID)
}

// RequestChat: channel container to communicate with this peer
func (pi *PeerInfo) RequestChat(ctx context.Context, cid ChannelID) (ChatWithPeer, error) {
	chat := newTalker(pi, cid)
	select {
	case pi.requestedSessions <- chat:
		return chat, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
