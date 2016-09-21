package directory

import (
	"fmt"
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

	// Comm interface for messages from GW -> peer
	tx            chan interface{}
	stateNotifier chan<- stateChange //< Needs external initialization
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

func (pi *PeerInfo) SubscribeCmdChan() <-chan interface{} {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	// We assume that the previous chan was closed via UnsubscribeCmdChan().
	log.Printf("[%s] SubscribeCmdChan ", pi.ID)
	if pi.tx == nil {
		pi.tx = make(chan interface{}, 1)
		log.Println("Create new comm chan directed to peer", pi.ID)
	}
	// Add peer to pool of available peers
	pi.stateNotifier <- stateChange{avail: 2, p: pi}

	// XXX: test sending some messages
	// go func() {
	// 	time.Sleep(time.Second * 2)
	// 	for i := 2; i >= 0; i-- {
	// 		prep := proto.SMCCmd_Prepare{&proto.Prepare{
	// 			Participants: []*proto.Prepare_Participant{&proto.Prepare_Participant{Addr: "myAddr", Identity: "ident"}},
	// 		}}
	// 		d.BroadcastMsg([]auth.PeerID{"sn3.flexsmc.local", "sn414141.flexsmc.local"}, proto.SMCCmd{Type: proto.SMCCmd_Type(i), Payload: &prep})
	// 	}
	// }()
	return pi.tx
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
	log.Println("Close comm chan to peer", pi.ID)
}

func (pi *PeerInfo) SendMsg(m interface{}) error {
	select {
	case pi.tx <- m:
		// No previous message in buffer. So we conclude that our previously queued message reached
		// its destination peer.
		return nil
	default:
		// Sender blocking (chan len = 1). A previously queued message is still waiting until
		// a specified delivery deadline rejects this message.
		return fmt.Errorf("delivery of last msg not succeeded yet (node temporarily offline?)")
	}
}
