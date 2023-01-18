package rovyapi

import (
	"net/netip"
	"os"
	"time"

	rovy "go.rovy.net"
)

type NodeInfo struct {
	PeerID    rovy.PeerID
	IPAddress netip.Addr
	Running   bool
}

type NodeAPI interface {
	Info() (NodeInfo, error)
	Stop() error
	Fcnet() FcnetAPI
	Peer() PeerAPI
	Discovery() DiscoveryAPI
}

type PeerStatus struct {
	Peers     []PeerInfo
	Listeners []PeerListener
}

type PeerInfo struct {
	PeerID rovy.PeerID
	Addr   rovy.Multiaddr
	Status string // ok, timeout, handshake-hello, connection-error, ...
	Reason string
}

type PeerListener struct {
	ListenAddr     rovy.Multiaddr   // what we told it to listen on
	EffectiveAddrs []rovy.Multiaddr // what it's actually listening on
	ExternalAddrs  []rovy.Multiaddr // what others might see (NAT, hole punching, ...)
}

type PeerAPI interface {
	Status() (PeerStatus, error)
	Listen(rovy.Multiaddr) (PeerListener, error)
	// Close(rovy.Multiaddr) (PeerListener, error)
	Connect(rovy.Multiaddr) (PeerInfo, error)
	// Disconnect(rovy.Multiaddr) (PeerInfo, error)
}

type DiscoveryStatus struct {
	LinkLocal DiscoveryLinkLocal
}

type DiscoveryLinkLocal struct {
	Interval time.Duration
}

type DiscoveryAPI interface {
	Status() (DiscoveryStatus, error)
	StartLinkLocal(DiscoveryLinkLocal) error
	StopLinkLocal() error
}

type FcnetAPI interface {
	Start(tunfd *os.File) error
	NodeAPI() NodeAPI // TODO: ?
}
