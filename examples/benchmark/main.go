package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"log"
	"os"
	"runtime/pprof"
	"time"

	multiaddr "github.com/multiformats/go-multiaddr"
	rovy "pkt.dev/go-rovy"
	node "pkt.dev/go-rovy/node"
)

const BenchmarkCodec = 0x42002

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

	nodeA.SessionManager().Multigram().AddCodec(BenchmarkCodec)
	nodeB.SessionManager().Multigram().AddCodec(BenchmarkCodec)

	if err = nodeA.Connect(nodeB.PeerID(), addrB); err != nil {
		return err
	}

	amount := 1000000
	mtu := rovy.PreliminaryMTU - 100 // XXX max currently seems to be 1433
	done := make(chan bool, 1)
	start := time.Now()

	var j int
	nodeB.Handle(BenchmarkCodec, func(p []byte, peerid rovy.PeerID, route rovy.Route) error {
		k, err := binary.ReadVarint(bytes.NewBuffer(p))
		if err != nil {
			log.Printf("ReadVarint: %s", err)
			return err
		}
		j += 1
		// log.Printf("k = %d", k)
		if int(k) >= amount {
			done <- true
		}
		return nil
	})

	nodeA.Log().Printf("sending %d packets, %d bytes each", amount, mtu)
	for i := 1; i <= amount; i++ {
		p := make([]byte, mtu)
		binary.PutVarint(p, int64(i))
		if err := nodeA.Send(nodeB.PeerID(), BenchmarkCodec, p); err != nil {
			return err
		}
	}

	<-done

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
