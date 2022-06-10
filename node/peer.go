package node

import (
	rovy "go.rovy.net"
	rovyapi "go.rovy.net/api"
)

type PeerAPI Node

func (c *PeerAPI) Status() rovyapi.PeerStatus {
	return rovyapi.PeerStatus{}
}

// TODO: implement dialers
func (c *PeerAPI) Enable(ma rovy.Multiaddr) (rovyapi.PeerDialer, error) {
	return rovyapi.PeerDialer{Protocol: ma, Enabled: true}, nil
}

// TODO: implement listeners
func (c *PeerAPI) Listen(ma rovy.Multiaddr) (rovyapi.PeerListener, error) {
	return rovyapi.PeerListener{Addr: ma}, nil
}

func (c *PeerAPI) Connect(ma rovy.Multiaddr) (rovyapi.PeerInfo, error) {
	return rovyapi.PeerInfo{}, nil
}

func (c *PeerAPI) NodeAPI() rovyapi.NodeAPI {
	return (*Node)(c)
}

var _ rovyapi.PeerAPI = &PeerAPI{}
