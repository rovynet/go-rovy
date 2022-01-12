package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"log"
	"net/netip"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	rovy "go.rovy.net"
	node "go.rovy.net/node"
)

const BenchmarkCodec = 0x42002

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

func run() error {
	cpuprof := flag.String("cpuprofile", "", "write cpu profile to `file`")
	flag.Parse()
	if *cpuprof != "" {
		f, err := os.Create(*cpuprof)
		if err != nil {
			return err
		}
		defer f.Close()
		err = pprof.StartCPUProfile(f)
		if err != nil {
			return nil
		}
		defer pprof.StopCPUProfile()
	}

	addrA := rovy.NewUDPMultiaddr(netip.MustParseAddrPort("[::1]:12345"))
	addrB := rovy.NewUDPMultiaddr(netip.MustParseAddrPort("[::1]:12346"))

	nodeA, err := newNode("nodeA", addrA)
	if err != nil {
		return err
	}
	nodeB, err := newNode("nodeB", addrB)
	if err != nil {
		return err
	}

	if err = nodeA.Connect(nodeB.PeerID(), addrB); err != nil {
		return err
	}

	amount := 1000000
	mtu := rovy.UpperMTU
	start := time.Now()

	var j int
	nodeB.Handle(BenchmarkCodec, func(pkt rovy.UpperPacket) error {
		_, err := binary.ReadVarint(bytes.NewBuffer(pkt.Payload()))
		if err != nil {
			log.Printf("ReadVarint: %s", err)
			return err
		}
		j += 1
		return nil
	})

	nodeA.Log().Printf("sending %d packets, %d bytes each", amount, mtu)
	for i := 1; i <= amount; i++ {
		p := make([]byte, mtu)
		binary.PutVarint(p, int64(i))
		if err := nodeA.Send(nodeB.PeerID(), BenchmarkCodec, p); err != nil {
			return err
		}
		runtime.Gosched()
	}

	time.Sleep(250 * time.Millisecond)

	duration := time.Now().Sub(start)
	gbps := float64(j*mtu) * 8 / 1024 / 1024 / 1024 / duration.Seconds()
	nodeB.Log().Printf("received %d packets, took %s, %.2f Gbps", j, duration, gbps)
	nodeB.Log().Printf("RxTpt=%d RxLower=%d RxUpper=%d", nodeB.RxTpt, nodeB.RxLower, nodeB.RxUpper)

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf(err.Error())
	}
}
