package node

import (
	rovyapi "go.rovy.net/api"
)

func (node *Node) Info() (rovyapi.NodeInfo, error) {
	return rovyapi.NodeInfo{node.PeerID()}, nil
}

var _ rovyapi.NodeAPI = &Node{}
