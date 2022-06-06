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

func (node *Node) Fc00() rovyapi.Fc00API {
	return nil // not implemented here
}

var _ rovyapi.NodeAPI = &Node{}
