package node

import (
	rovy "go.rovy.net"
	rovyapi "go.rovy.net/api"
)

type PeerAPI Node

func (c *PeerAPI) Status() (rovyapi.PeerStatus, error) {
	var listeners []rovyapi.PeerListener
	for _, tpt := range (*Node)(c).transports {
		listeners = append(listeners, rovyapi.PeerListener{ListenAddr: tpt.LocalMultiaddr()})
	}
	return rovyapi.PeerStatus{Listeners: listeners}, nil
}

func (c *PeerAPI) Listen(ma rovy.Multiaddr) (rovyapi.PeerListener, error) {
	tpt, err := NewTransport(ma, c.logger)
	if err != nil {
		return rovyapi.PeerListener{}, err
	}

	node := (*Node)(c)
	node.transports = append(node.transports, tpt)
	if ((*Node)(c)).Running() {
		tpt.Start(node.lowerRecvQ)
	}

	return rovyapi.PeerListener{ListenAddr: ma}, nil
}

func (c *PeerAPI) Connect(ma rovy.Multiaddr) (rovyapi.PeerInfo, error) {
	return rovyapi.PeerInfo{}, nil
}

func (c *PeerAPI) NodeAPI() rovyapi.NodeAPI {
	return (*Node)(c)
}

var _ rovyapi.PeerAPI = &PeerAPI{}
