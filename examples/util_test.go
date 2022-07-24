package examples_test

import (
	"log"
	"os"

	rovy "go.rovy.net"
	node "go.rovy.net/node"
)

func newNode(name string, lisaddr rovy.Multiaddr) (*node.Node, error) {
	logger := log.New(os.Stderr, "["+name+"] ", log.Ltime|log.Lshortfile)

	node := node.NewNode(rovy.MustGeneratePrivateKey(), logger)
	if err := node.Start(); err != nil {
		return node, err
	}

	if _, err := node.Peer().Listen(lisaddr); err != nil {
		return nil, err
	}

	logger.Printf("%s/rovy/%s", lisaddr, node.PeerID())
	return node, nil
}
