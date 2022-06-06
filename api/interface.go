package rovyapi

import (
	"os"

	rovy "go.rovy.net"
)

type NodeInfo struct {
	PeerID rovy.PeerID
}

// type PeerInfo struct {
// 	PeerID rovy.PeerID
// 	Addr   rovy.UDPMultiaddr
// 	Addrs  []rovy.UDPMultiaddr
// }

type NodeAPI interface {
	Info() (NodeInfo, error)
	Stop() error
	Fc00() Fc00API
}

type Fc00API interface {
	Start(tunfd *os.File) error
	NodeAPI() NodeAPI
}
