package examples_test

import (
	"log"
	"os"

	rovy "go.rovy.net"
	node "go.rovy.net/node"
)

func newNode(name string, lisaddr rovy.UDPMultiaddr) (*node.Node, error) {
	logger := log.New(os.Stderr, "["+name+"] ", log.Ltime|log.Lshortfile)

	privkey, err := rovy.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}

	node := node.NewNode(privkey, logger)

	if err = node.Listen(lisaddr); err != nil {
		return nil, err
	}

	logger.Printf("%s/rovy/%s", lisaddr, node.PeerID())
	return node, nil
}
