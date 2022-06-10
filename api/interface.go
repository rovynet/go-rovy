package rovyapi

import (
	"os"

	rovy "go.rovy.net"
)

type NodeInfo struct {
	PeerID rovy.PeerID
}

type NodeAPI interface {
	Info() (NodeInfo, error)
	Stop() error
	Fc00() Fc00API
	Peer() PeerAPI
}

type PeerStatus struct {
	Peers     []PeerInfo
	Listeners []PeerListener
	Dialers   []PeerDialer
}

type PeerInfo struct {
	PeerID rovy.PeerID
	Addr   rovy.Multiaddr
	Status string // ok, timeout, handshake-hello, connection-error, ...
	Reason string
}

type PeerListener struct {
	Addr           rovy.Multiaddr   // what we told it to listen on
	EffectiveAddrs []rovy.Multiaddr // what it's actually listening on
	ExternalAddrs  []rovy.Multiaddr // what others might see (NAT, hole punching, ...)
}

type PeerDialer struct {
	Protocol rovy.Multiaddr // the /maddrfmt address
	Enabled  bool
}

type PeerAPI interface {
	Status() PeerStatus
	Enable(rovy.Multiaddr) (PeerDialer, error)
	// Disable(rovy.Multiaddr) (PeerDialer, error)
	Listen(rovy.Multiaddr) (PeerListener, error)
	// Close(rovy.Multiaddr) (PeerListener, error)
	Connect(rovy.Multiaddr) (PeerInfo, error)
	// Disconnect(rovy.Multiaddr) (PeerInfo, error)
}

type Fc00API interface {
	Start(tunfd *os.File) error
	NodeAPI() NodeAPI
}
