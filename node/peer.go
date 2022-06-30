package node

import (
	rovy "go.rovy.net"
	rovyapi "go.rovy.net/api"
)

type PeerAPI Node

func (c *PeerAPI) Status() rovyapi.PeerStatus {
	return rovyapi.PeerStatus{}
}

func (c *PeerAPI) Listen(ma rovy.Multiaddr) (rovyapi.PeerListener, error) {
	tpt, err := NewTransport(ma, c.logger)
	if err != nil {
		return rovyapi.PeerListener{}, err
	}

	node := (*Node)(c)
	node.transports = append(node.transports, tpt)
	go tpt.RecvRoutine(node.lowerRecvQ)
	go tpt.SendRoutine()

	return rovyapi.PeerListener{ListenAddr: ma}, nil
}

func (c *PeerAPI) Connect(ma rovy.Multiaddr) (rovyapi.PeerInfo, error) {
	return rovyapi.PeerInfo{}, nil
}

func (c *PeerAPI) Policy(pols ...string) error {
	return nil
}

func (c *PeerAPI) NodeAPI() rovyapi.NodeAPI {
	return (*Node)(c)
}

var _ rovyapi.PeerAPI = &PeerAPI{}
