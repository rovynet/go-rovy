package main

import (
	"log"
	"os"
	"time"

	multiaddr "github.com/multiformats/go-multiaddr"
	rovy "pkt.dev/go-rovy"
	node "pkt.dev/go-rovy/node"
)

const EchoMulticodec = 0x42001

func newNode(name string, lisaddr multiaddr.Multiaddr) (*node.Node, error) {
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

func run() error {
	addrA := multiaddr.StringCast("/ip6/::1/udp/12345")
	addrB := multiaddr.StringCast("/ip6/::1/udp/12346")

	nodeA, err := newNode("nodeA", addrA)
	if err != nil {
		return err
	}
	nodeB, err := newNode("nodeB", addrB)
	if err != nil {
		return err
	}

	nodeB.Handle(EchoMulticodec, func(p []byte, peerid rovy.PeerID, route rovy.Route) error {
		nodeB.Log().Printf("got packet from %s %#v", peerid, p)

		if err := nodeB.Send(peerid, EchoMulticodec, p); err != nil {
			nodeB.Log().Printf("send: %s", err)
		}

		return nil
	})
	nodeA.Handle(EchoMulticodec, func(p []byte, peerid rovy.PeerID, route rovy.Route) error {
		nodeA.Log().Printf("got echo %#v", p)
		return nil
	})

	if err := nodeA.Connect(nodeB.PeerID(), addrB); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeA.Send(nodeB.PeerID(), EchoMulticodec, []byte{0x42, 0x42, 0x42, 0x42}); err != nil {
		nodeA.Log().Printf("failed to send to nodeB: %s", err)
		return err
	}

	time.Sleep(500 * time.Millisecond)

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf(err.Error())
	}
}
