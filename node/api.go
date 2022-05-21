package node

import (
	rovyapi "go.rovy.net/api"
)

func (node *Node) Info() (rovyapi.NodeInfo, error) {
	return rovyapi.NodeInfo{node.PeerID()}, nil
}

func (node *Node) Stop() error {
	return nil
}

var _ rovyapi.NodeAPI = &Node{}
