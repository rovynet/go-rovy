package rovyapi

import (
	"os"

	rovy "go.rovy.net"
)

type NodeInfo struct {
	PeerID rovy.PeerID
}

type PeerInfo struct {
	PeerID rovy.PeerID
	Addr   rovy.UDPMultiaddr
	Addrs  []rovy.UDPMultiaddr
}

type NodeAPI interface {
	Info() (NodeInfo, error)
	Stop() error

	// Why do we keep the fc00 API out of *the* API?
	// Because complexity avoidance. The fc00 thing *will* change a lot
	// and that's why for now it's best to avoid accidental interdependencies.
	// We'll try to keep all fc00 code as self-contained and separate as possible.
	// Fc00API() Fc00API
}

type Fc00API interface {
	Start(tunfd *os.File) error
	NodeAPI() NodeAPI
}
