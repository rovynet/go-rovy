package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"net"
	"os"
	"time"

	multiaddr "github.com/multiformats/go-multiaddr"
	blake2s "golang.org/x/crypto/blake2s"
	rovy "pkt.dev/go-rovy"
	node "pkt.dev/go-rovy/node"
)

var (
	IPv6Prefix = []byte{0xfc}
)

func genKey(prefix []byte) (rovy.PrivateKey, rovy.PublicKey, error) {
	for {
		var pubkey rovy.PublicKey

		privkey, err := rovy.NewPrivateKey()
		if err != nil {
			return privkey, pubkey, err
		}

		pubkey = privkey.PublicKey()

		ipv6 := ipv6FromKey(pubkey)
		if bytes.Equal(ipv6[0:len(prefix)], prefix) {
			return privkey, pubkey, nil
		}
	}
}

func ipv6FromKey(pubkey rovy.PublicKey) net.IP {
	hash := blake2s.Sum256(pubkey[:])
	h := blake2s.Sum256(hash[:])
	return net.IP(h[16:32])
}

func newNode(name string, lisaddr multiaddr.Multiaddr) (*node.Node, error) {
	privkey, pubkey, err := genKey(IPv6Prefix)
	if err != nil {
		return nil, err
	}

	logger := log.New(os.Stderr, name+" -- ", log.LstdFlags)

	node := node.NewNode(privkey, pubkey, logger)

	if err = node.Listen(lisaddr); err != nil {
		return nil, err
	}

	logger.Printf("/ip6/%s", ipv6FromKey(pubkey))
	logger.Printf("/rovy/%s", node.PeerID())
	logger.Printf("listen %s", lisaddr)
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

	return runHello(nodeA, nodeB)
	// return runBench(nodeA, nodeB)
}

func runHello(nodeA *node.Node, nodeB *node.Node) error {
	nodeA.Handle(func(p []byte, peerid rovy.PeerID) {
		nodeA.Log().Printf("got data from %s: %+v", peerid, p)
	})
	nodeB.Handle(func(p []byte, peerid rovy.PeerID) {
		nodeB.Log().Printf("got data from %s: %+v", peerid, p)
	})

	if err := nodeA.Send(nodeB.PeerID(), []byte{0x1, 0x3, 0x1, 0x2}); err != nil {
		return err
	}

	if err := nodeB.Send(nodeA.PeerID(), []byte{0x1, 0x3, 0x1, 0x2}); err != nil {
		return err
	}

	time.Sleep(500 * time.Millisecond)

	return nil
}

func runBench(nodeA *node.Node, nodeB *node.Node) error {
	amount := 100000
	done := make(chan bool, 1)
	start := time.Now()

	var j int
	nodeB.Handle(func(p []byte, peerid rovy.PeerID) {
		k, err := binary.ReadVarint(bytes.NewBuffer(p))
		if err != nil {
			log.Printf("ReadVarint: %s", err)
		}
		j += 1
		// log.Printf("k = %d", k)
		if int(k) == amount {
			done <- true
		}
	})

	for i := 1; i <= amount; i++ {
		p := make([]byte, rovy.PreliminaryMTU-100)
		binary.PutVarint(p, int64(i))
		if err := nodeA.Send(nodeB.PeerID(), p); err != nil {
			return err
		}
	}

	// _ = done
	<-done
	log.Printf("done, packets: %d, time: %s", j, time.Now().Sub(start))

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf(err.Error())
	}
}
