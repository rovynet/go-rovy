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

	wgnet "golang.zx2c4.com/wireguard/tun/netstack"

	rovy "go.rovy.net"
	fcnet "go.rovy.net/fcnet"
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

	lisaddr.PeerID = node.PeerID()
	logger.Printf("%s", lisaddr)
	logger.Printf("%s", node.IPAddr())
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

	nmA := fcnet.NewNMTUN(nodeA.Log())
	if err := nmA.Start("rovy0", nodeA.IPAddr(), rovy.UpperMTU); err != nil {
		return err
	}
	devA := nmA.Device()

	dnssrv := []netip.Addr{netip.MustParseAddr("fc00::1")}

	devB, _, err := wgnet.CreateNetTUN([]netip.Addr{nodeB.IPAddr()}, dnssrv, rovy.UpperMTU)
	if err != nil {
		return err
	}

	devC, _, err := wgnet.CreateNetTUN([]netip.Addr{nodeC.IPAddr()}, dnssrv, rovy.UpperMTU)
	if err != nil {
		return err
	}

	devD, netD, err := wgnet.CreateNetTUN([]netip.Addr{nodeD.IPAddr()}, dnssrv, rovy.UpperMTU)
	if err != nil {
		return err
	}

	fcnetA := fcnet.NewFcnet(nodeA, devA)
	if err := fcnetA.Start(rovy.UpperMTU); err != nil {
		return err
	}

	fcnetB := fcnet.NewFcnet(nodeB, devB)
	if err := fcnetB.Start(rovy.UpperMTU); err != nil {
		return err
	}

	fcnetC := fcnet.NewFcnet(nodeC, devC)
	if err := fcnetC.Start(rovy.UpperMTU); err != nil {
		return err
	}

	fcnetD := fcnet.NewFcnet(nodeD, devD)
	if err := fcnetD.Start(rovy.UpperMTU); err != nil {
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
	nodeD.Log().Printf("open http://%s.rovy", nodeD.PeerID())
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, fmt.Sprintf("Hello from\n%s\n", nodeD.IPAddr()))
			for _, ma := range nodeB.Addresses() {
				io.WriteString(w, ma.String())
			}
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
