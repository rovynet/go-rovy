package node

import (
	rovyapi "go.rovy.net/api"
)

func (node *Node) Info() (rovyapi.NodeInfo, error) {
	return rovyapi.NodeInfo{PeerID: node.PeerID()}, nil
}

func (node *Node) Stop() error {
	return nil
}

func (node *Node) Fcnet() rovyapi.FcnetAPI {
	return nil // not implemented here
}

func (node *Node) Peer() rovyapi.PeerAPI {
	return (*PeerAPI)(node)
}

var _ rovyapi.NodeAPI = &Node{}
