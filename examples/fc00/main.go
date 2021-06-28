package main

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	multiaddr "github.com/multiformats/go-multiaddr"

	rovy "pkt.dev/go-rovy"
	fc00 "pkt.dev/go-rovy/fc00"
	node "pkt.dev/go-rovy/node"
)

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
	logger.Printf("/ip6/%s", node.PeerID().PublicKey().Addr())
	return node, nil
}

func run() error {
	addrA := multiaddr.StringCast("/ip6/::1/udp/12345")
	addrB := multiaddr.StringCast("/ip6/::1/udp/12346")
	addrC := multiaddr.StringCast("/ip6/::1/udp/12347")
	addrD := multiaddr.StringCast("/ip6/::1/udp/12348")

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

	devA, err := fc00.DefaultTun("rovy0", nodeA.PeerID().PublicKey().Addr(), rovy.PreliminaryMTU, nodeA.Log())
	if err != nil {
		return err
	}

	devB, err := fc00.DefaultTun("rovy1", nodeB.PeerID().PublicKey().Addr(), rovy.PreliminaryMTU, nodeB.Log())
	if err != nil {
		return err
	}

	devC, err := fc00.DefaultTun("rovy2", nodeC.PeerID().PublicKey().Addr(), rovy.PreliminaryMTU, nodeC.Log())
	if err != nil {
		return err
	}

	devD, err := fc00.DefaultTunWithNamespace("rovy3", nodeD.PeerID().PublicKey().Addr(), rovy.PreliminaryMTU, nodeD.Log())
	if err != nil {
		return err
	}

	fc00a := fc00.NewFc00(nodeA, devA, nodeA.Routing())
	if err := fc00a.Start(); err != nil {
		return err
	}

	fc00b := fc00.NewFc00(nodeB, devB, nodeB.Routing())
	if err := fc00b.Start(); err != nil {
		return err
	}

	fc00c := fc00.NewFc00(nodeC, devC, nodeC.Routing())
	if err := fc00c.Start(); err != nil {
		return err
	}

	fc00d := fc00.NewFc00(nodeD, devD, nodeD.Routing())
	if err := fc00d.Start(); err != nil {
		return err
	}

	if err := nodeA.Connect(nodeB.PeerID(), addrB); err != nil {
		nodeA.Log().Printf("failed to connect nodeA to nodeB: %s", err)
		return err
	}
	if err := nodeB.Connect(nodeC.PeerID(), addrC); err != nil {
		nodeB.Log().Printf("failed to connect nodeB to nodeC: %s", err)
		return err
	}
	if err := nodeC.Connect(nodeD.PeerID(), addrD); err != nil {
		nodeC.Log().Printf("failed to connect nodeC to nodeD: %s", err)
		return err
	}

	// A->B->C->D
	nodeA.Routing().AddRoute(nodeD.PeerID(),
		nodeA.Routing().MustGetRoute(nodeB.PeerID()).
			Join(nodeB.Routing().MustGetRoute(nodeC.PeerID())).
			Join(nodeC.Routing().MustGetRoute(nodeD.PeerID())))
	if err := nodeA.Connect(nodeD.PeerID(), nil); err != nil {
		nodeA.Log().Printf("failed to connect nodeA to nodeD: %s", err)
		return err
	}

	nodeA.Routing().PrintTable(nodeA.Log())

	select {}
	return nil
}

func main() {
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGQUIT)
		buf := make([]byte, 1<<20)
		for {
			<-sigs
			stacklen := runtime.Stack(buf, true)
			log.Printf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end\n", buf[:stacklen])
		}
	}()

	if err := run(); err != nil {
		log.Fatalf(err.Error())
	}
}
