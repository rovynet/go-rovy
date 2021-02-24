package main

import (
	"fmt"
	"log"
	"os"
	"time"

	multiaddr "github.com/multiformats/go-multiaddr"
	rovy "pkt.dev/go-rovy"
	node "pkt.dev/go-rovy/node"
)

const RoutedCodec = 42003

func newNode(name string, lisaddr multiaddr.Multiaddr) (*node.Node, error) {
	logger := log.New(os.Stderr, "["+name+"] ", log.Ltime|log.Lshortfile)

	privkey, err := rovy.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	node := node.NewNode(privkey, logger)

	node.Handle(RoutedCodec, func(buf []byte, from rovy.PeerID) {
		node.Log().Printf("got packet from %s: %#v", from, buf)
	})

	if err = node.Listen(lisaddr); err != nil {
		return nil, err
	}

	logger.Printf("%s/rovy/%s", lisaddr, node.PeerID())
	return node, nil
}

func run() error {
	addrA := multiaddr.StringCast("/ip4/127.0.0.1/udp/12340")
	addrB := multiaddr.StringCast("/ip4/127.0.0.1/udp/12341")
	addrC := multiaddr.StringCast("/ip4/127.0.0.1/udp/12342")
	addrD := multiaddr.StringCast("/ip4/127.0.0.1/udp/12343")
	addrE := multiaddr.StringCast("/ip4/127.0.0.1/udp/12344")
	addrF := multiaddr.StringCast("/ip4/127.0.0.1/udp/12345")
	addrG := multiaddr.StringCast("/ip4/127.0.0.1/udp/12346")
	addrH := multiaddr.StringCast("/ip4/127.0.0.1/udp/12347")
	addrI := multiaddr.StringCast("/ip4/127.0.0.1/udp/12348")
	addrJ := multiaddr.StringCast("/ip4/127.0.0.1/udp/12349")
	addrK := multiaddr.StringCast("/ip4/127.0.0.1/udp/12350")

	nodeA, err := newNode("nodeA", addrA)
	if err != nil {
		return err
	}
	nodeB, err := newNode("nodeB", addrB)
	if err != nil {
		return err
	}
	nodeC, err := newNode("nodeC", addrC)
	if err != nil {
		return err
	}
	nodeD, err := newNode("nodeD", addrD)
	if err != nil {
		return err
	}
	nodeE, err := newNode("nodeE", addrE)
	if err != nil {
		return err
	}
	nodeF, err := newNode("nodeF", addrF)
	if err != nil {
		return err
	}
	nodeG, err := newNode("nodeG", addrG)
	if err != nil {
		return err
	}
	nodeH, err := newNode("nodeH", addrH)
	if err != nil {
		return err
	}
	nodeI, err := newNode("nodeI", addrI)
	if err != nil {
		return err
	}
	nodeJ, err := newNode("nodeJ", addrJ)
	if err != nil {
		return err
	}
	nodeK, err := newNode("nodeK", addrK)
	if err != nil {
		return err
	}

	// A <-> B/C/D
	if err := nodeA.Connect(nodeB.PeerID(), addrB); err != nil {
		nodeA.Log().Printf("failed to connect nodeA to nodeB: %s", err)
		return err
	}
	if err := nodeA.Connect(nodeC.PeerID(), addrC); err != nil {
		nodeA.Log().Printf("failed to connect nodeA to nodeC: %s", err)
		return err
	}
	if err := nodeA.Connect(nodeD.PeerID(), addrD); err != nil {
		nodeA.Log().Printf("failed to connect nodeA to nodeD: %s", err)
		return err
	}

	// B/C/D <-> E/F/G
	if err := nodeB.Connect(nodeE.PeerID(), addrE); err != nil {
		nodeA.Log().Printf("failed to connect nodeB to nodeE: %s", err)
		return err
	}
	if err := nodeB.Connect(nodeF.PeerID(), addrF); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeB.Connect(nodeG.PeerID(), addrG); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeC.Connect(nodeE.PeerID(), addrE); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeC.Connect(nodeF.PeerID(), addrF); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeC.Connect(nodeG.PeerID(), addrG); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeD.Connect(nodeE.PeerID(), addrE); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeD.Connect(nodeF.PeerID(), addrF); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeD.Connect(nodeG.PeerID(), addrG); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}

	// E/F/G <-> H/I/J
	if err := nodeE.Connect(nodeH.PeerID(), addrH); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeE.Connect(nodeI.PeerID(), addrI); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeE.Connect(nodeJ.PeerID(), addrJ); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeF.Connect(nodeH.PeerID(), addrH); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeF.Connect(nodeI.PeerID(), addrI); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeF.Connect(nodeJ.PeerID(), addrJ); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeG.Connect(nodeH.PeerID(), addrH); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeG.Connect(nodeI.PeerID(), addrI); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}
	if err := nodeG.Connect(nodeJ.PeerID(), addrJ); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		return err
	}

	// H/I/J <-> K
	if err := nodeH.Connect(nodeK.PeerID(), addrK); err != nil {
		nodeA.Log().Printf("failed to connect nodeH to nodeK: %s", err)
		return err
	}
	if err := nodeI.Connect(nodeK.PeerID(), addrK); err != nil {
		nodeA.Log().Printf("failed to connect nodeI to nodeK: %s", err)
		return err
	}
	if err := nodeJ.Connect(nodeK.PeerID(), addrK); err != nil {
		nodeA.Log().Printf("failed to connect nodeJ to nodeK: %s", err)
		return err
	}

	time.Sleep(250 * time.Millisecond)

	// actual routes
	// A     <-> B/C/D
	// B/C/D <-> E/F/G
	// E/F/G <-> H/I/J
	// H/I/J <-> K

	// A->B->F->J->K
	nodeA.Routing().AddRoute(nodeK.PeerID(),
		nodeA.Routing().MustGetRoute(nodeB.PeerID()).
			Join(nodeB.Routing().MustGetRoute(nodeF.PeerID())).
			Join(nodeF.Routing().MustGetRoute(nodeJ.PeerID())).
			Join(nodeJ.Routing().MustGetRoute(nodeK.PeerID())))

	nodeA.Routing().PrintTable(nodeA.Log())

	if err := nodeA.Send(nodeK.PeerID(), RoutedCodec, []byte{0x42, 0x42, 0x42, 0x42}); err != nil {
		nodeA.Log().Printf("failed to send nodeA -> nodeK: %s", err)
		return err
	}

	time.Sleep(250 * time.Millisecond)

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}
