package examples_test

import (
	"bytes"
	"testing"
	"time"

	multiaddr "github.com/multiformats/go-multiaddr"
	rovy "pkt.dev/go-rovy"
)

func TestEcho(t *testing.T) {
	codec := uint64(0x42001)

	payload := []byte{0x42, 0x42, 0x42, 0x42}
	payload2 := []byte{0x0, 0x0, 0x0, 0x0}

	addrA := multiaddr.StringCast("/ip6/::1/udp/12345")
	addrB := multiaddr.StringCast("/ip6/::1/udp/12346")

	nodeA, err := newNode("nodeA", addrA)
	if err != nil {
		t.Error(err)
		return
	}
	nodeB, err := newNode("nodeB", addrB)
	if err != nil {
		t.Error(err)
		return
	}

	nodeB.Handle(codec, func(pkt rovy.UpperPacket) error {
		pl := pkt.Payload()
		nodeB.Log().Printf("got packet from %s (len=%d) %#v", pkt.UpperSrc, len(pl), pl)

		if err := nodeB.Send(pkt.UpperSrc, codec, pkt.Payload()); err != nil {
			nodeB.Log().Printf("send: %s", err)
		}

		return nil
	})

	nodeA.Handle(codec, func(pkt rovy.UpperPacket) error {
		pl := pkt.Payload()
		copy(payload2[:0], pl)

		nodeA.Log().Printf("got echo from %s (len=%d) %#v", pkt.UpperSrc, len(pl), pl)
		return nil
	})

	if err := nodeA.Connect(nodeB.PeerID(), addrB); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}

	// setup done, let's go

	if err := nodeA.Send(nodeB.PeerID(), codec, payload); err != nil {
		nodeA.Log().Printf("failed to send to nodeB: %s", err)
		t.Error(err)
		return
	}

	time.Sleep(100 * time.Millisecond)

	if bytes.Equal(payload2, payload) {
		t.Fatalf("expected %#v but got %#v", payload, payload2)
		return
	}
}
