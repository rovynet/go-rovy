package examples_test

import (
	"bytes"
	"testing"
	"time"

	multiaddr "github.com/multiformats/go-multiaddr"
	rovy "github.com/rovynet/go-rovy"
)

func TestRouted(t *testing.T) {
	codec := uint64(0x42003)

	payload := []byte{0x42, 0x42, 0x42, 0x42}
	payload2 := []byte{0x0, 0x0, 0x0, 0x0}

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
		t.Error(err)
		return
	}
	nodeB, err := newNode("nodeB", addrB)
	if err != nil {
		t.Error(err)
		return
	}
	nodeC, err := newNode("nodeC", addrC)
	if err != nil {
		t.Error(err)
		return
	}
	nodeD, err := newNode("nodeD", addrD)
	if err != nil {
		t.Error(err)
		return
	}
	nodeE, err := newNode("nodeE", addrE)
	if err != nil {
		t.Error(err)
		return
	}
	nodeF, err := newNode("nodeF", addrF)
	if err != nil {
		t.Error(err)
		return
	}
	nodeG, err := newNode("nodeG", addrG)
	if err != nil {
		t.Error(err)
		return
	}
	nodeH, err := newNode("nodeH", addrH)
	if err != nil {
		t.Error(err)
		return
	}
	nodeI, err := newNode("nodeI", addrI)
	if err != nil {
		t.Error(err)
		return
	}
	nodeJ, err := newNode("nodeJ", addrJ)
	if err != nil {
		t.Error(err)
		return
	}
	nodeK, err := newNode("nodeK", addrK)
	if err != nil {
		t.Error(err)
		return
	}

	// A <-> B/C/D
	if err := nodeA.Connect(nodeB.PeerID(), addrB); err != nil {
		nodeA.Log().Printf("failed to connect nodeA to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeA.Connect(nodeC.PeerID(), addrC); err != nil {
		nodeA.Log().Printf("failed to connect nodeA to nodeC: %s", err)
		t.Error(err)
		return
	}
	if err := nodeA.Connect(nodeD.PeerID(), addrD); err != nil {
		nodeA.Log().Printf("failed to connect nodeA to nodeD: %s", err)
		t.Error(err)
		return
	}

	// B/C/D <-> E/F/G
	if err := nodeB.Connect(nodeE.PeerID(), addrE); err != nil {
		nodeA.Log().Printf("failed to connect nodeB to nodeE: %s", err)
		t.Error(err)
		return
	}
	if err := nodeB.Connect(nodeF.PeerID(), addrF); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeB.Connect(nodeG.PeerID(), addrG); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeC.Connect(nodeE.PeerID(), addrE); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeC.Connect(nodeF.PeerID(), addrF); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeC.Connect(nodeG.PeerID(), addrG); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeD.Connect(nodeE.PeerID(), addrE); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeD.Connect(nodeF.PeerID(), addrF); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeD.Connect(nodeG.PeerID(), addrG); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}

	// E/F/G <-> H/I/J
	if err := nodeE.Connect(nodeH.PeerID(), addrH); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeE.Connect(nodeI.PeerID(), addrI); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeE.Connect(nodeJ.PeerID(), addrJ); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeF.Connect(nodeH.PeerID(), addrH); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeF.Connect(nodeI.PeerID(), addrI); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeF.Connect(nodeJ.PeerID(), addrJ); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeG.Connect(nodeH.PeerID(), addrH); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeG.Connect(nodeI.PeerID(), addrI); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}
	if err := nodeG.Connect(nodeJ.PeerID(), addrJ); err != nil {
		nodeA.Log().Printf("failed to connect to nodeB: %s", err)
		t.Error(err)
		return
	}

	// H/I/J <-> K
	if err := nodeH.Connect(nodeK.PeerID(), addrK); err != nil {
		nodeA.Log().Printf("failed to connect nodeH to nodeK: %s", err)
		t.Error(err)
		return
	}
	if err := nodeI.Connect(nodeK.PeerID(), addrK); err != nil {
		nodeA.Log().Printf("failed to connect nodeI to nodeK: %s", err)
		t.Error(err)
		return
	}
	if err := nodeJ.Connect(nodeK.PeerID(), addrK); err != nil {
		nodeA.Log().Printf("failed to connect nodeJ to nodeK: %s", err)
		t.Error(err)
		return
	}

	nodeA.Handle(codec, func(pkt rovy.UpperPacket) error {
		nodeA.Log().Printf("got packet from %s: %#v", pkt.UpperSrc, pkt.Payload())
		return nil
	})
	nodeK.Handle(codec, func(pkt rovy.UpperPacket) error {
		pl := pkt.Payload()
		copy(payload2, pl)

		nodeK.Log().Printf("got packet from %s: %#v", pkt.UpperSrc, pl)
		return nil
	})

	time.Sleep(100 * time.Millisecond)

	// peerings
	// A     <-> B/C/D
	// B/C/D <-> E/F/G
	// E/F/G <-> H/I/J
	// H/I/J <-> K

	// route
	// A->B->F->J->K
	nodeA.Routing().AddRoute(nodeK.PeerID(),
		nodeA.Routing().MustGetRoute(nodeB.PeerID()).
			Join(nodeB.Routing().MustGetRoute(nodeF.PeerID())).
			Join(nodeF.Routing().MustGetRoute(nodeJ.PeerID())).
			Join(nodeJ.Routing().MustGetRoute(nodeK.PeerID())))

	// nodeA.Routing().PrintTable(nodeA.Log())

	if err := nodeA.Connect(nodeK.PeerID(), nil); err != nil {
		nodeA.Log().Printf("failed to connect nodeA to nodeK: %s", err)
		t.Error(err)
		return
	}

	if err := nodeA.Send(nodeK.PeerID(), codec, payload); err != nil {
		nodeA.Log().Printf("failed to send nodeA -> nodeK: %s", err)
		t.Error(err)
		return
	}

	time.Sleep(100 * time.Millisecond)

	if !bytes.Equal(payload2, payload) {
		t.Fatalf("expected %#v but got %#v", payload, payload2)
		return
	}
}
