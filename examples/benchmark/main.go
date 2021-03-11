package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"os"
	"time"

	multiaddr "github.com/multiformats/go-multiaddr"
	rovy "pkt.dev/go-rovy"
	node "pkt.dev/go-rovy/node"
)

const BenchmarkCodec = 0x42002

func newNode(name string, lisaddr multiaddr.Multiaddr) (*node.Node, error) {
	logger := log.New(os.Stderr, "["+name+"] ", log.Ltime|log.Lshortfile)

	privkey, err := rovy.NewPrivateKey()
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
	addrA := multiaddr.StringCast("/ip4/127.0.0.1/udp/12345")
	addrB := multiaddr.StringCast("/ip4/127.0.0.1/udp/12346")

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
	mtu := rovy.PreliminaryMTU - 100
	done := make(chan bool, 1)
	start := time.Now()

	var j int
	nodeB.Handle(BenchmarkCodec, func(p []byte, peerid rovy.PeerID) error {
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

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf(err.Error())
	}
}
