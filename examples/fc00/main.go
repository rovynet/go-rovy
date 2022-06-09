package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	rovy "go.rovy.net"
	fc00 "go.rovy.net/fc00"
	rovygvisor "go.rovy.net/fc00/gvisor"
	node "go.rovy.net/node"
)

func newNode(name string, lisaddr rovy.Multiaddr) (*node.Node, error) {
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
	addrA := rovy.MustParseMultiaddr("/ip6/::1/udp/12345")
	addrB := rovy.MustParseMultiaddr("/ip6/::1/udp/12346")
	addrC := rovy.MustParseMultiaddr("/ip6/::1/udp/12347")
	addrD := rovy.MustParseMultiaddr("/ip6/::1/udp/12348")

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

	devA, err := fc00.NetworkManagerTun("rovy0", nodeA.PeerID().PublicKey().Addr(), rovy.UpperMTU, nodeA.Log())
	if err != nil {
		return err
	}

	ip6aB, _ := netip.AddrFromSlice([]byte(nodeB.PeerID().PublicKey().Addr()))
	devB, _, err := rovygvisor.NewGvisorTUN(ip6aB, rovy.UpperMTU, nodeB.Log())
	if err != nil {
		return err
	}

	ip6aC, _ := netip.AddrFromSlice([]byte(nodeC.PeerID().PublicKey().Addr()))
	devC, _, err := rovygvisor.NewGvisorTUN(ip6aC, rovy.UpperMTU, nodeC.Log())
	if err != nil {
		return err
	}

	ip6aD, _ := netip.AddrFromSlice([]byte(nodeD.PeerID().PublicKey().Addr()))
	devD, netD, err := rovygvisor.NewGvisorTUN(ip6aD, rovy.UpperMTU, nodeD.Log())
	if err != nil {
		return err
	}

	fc00a := fc00.NewFc00(nodeA, devA)
	if err := fc00a.Start(rovy.UpperMTU); err != nil {
		return err
	}

	fc00b := fc00.NewFc00(nodeB, devB)
	if err := fc00b.Start(rovy.UpperMTU); err != nil {
		return err
	}

	fc00c := fc00.NewFc00(nodeC, devC)
	if err := fc00c.Start(rovy.UpperMTU); err != nil {
		return err
	}

	fc00d := fc00.NewFc00(nodeD, devD)
	if err := fc00d.Start(rovy.UpperMTU); err != nil {
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
	if err := nodeA.Connect(nodeD.PeerID(), rovy.Multiaddr{}); err != nil {
		nodeA.Log().Printf("failed to connect nodeA to nodeD: %s", err)
		return err
	}

	nodeA.Routing().PrintTable(nodeA.Log())

	lis, err := netD.ListenTCP(&net.TCPAddr{Port: 80})
	if err != nil {
		return err
	}
	pid := nodeD.PeerID().String()
	ip6a := nodeD.PeerID().PublicKey().Addr().String()
	nodeD.Log().Printf("open http://%s.rovy", pid)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, fmt.Sprintf("Hello from\n/rovy/%s\n/ip6/%s\n", pid, ip6a))
		})
		if err = http.Serve(lis, mux); err != nil {
			nodeD.Log().Printf("http: %s", err)
			return
		}
	}()

	select {}
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
