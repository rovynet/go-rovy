package node

import (
	rovyapi "go.rovy.net/api"
)

func (node *Node) Info() (rovyapi.NodeInfo, error) {
	return rovyapi.NodeInfo{
		PeerID:    node.PeerID(),
		IPAddress: node.IPAddr(),
		Running:   node.Running(),
	}, nil
}

func (node *Node) Fcnet() rovyapi.FcnetAPI {
	return nil // not implemented here
}

func (node *Node) Peer() rovyapi.PeerAPI {
	return (*PeerAPI)(node)
}

func (node *Node) Discovery() rovyapi.DiscoveryAPI {
	return (*DiscoveryAPI)(node)
}

var _ rovyapi.NodeAPI = &Node{}
