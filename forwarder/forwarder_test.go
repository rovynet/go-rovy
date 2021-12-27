package forwarder_test

import (
	"io/ioutil"
	"log"
	"testing"

	rovy "go.rovy.net"
	forwarder "go.rovy.net/forwarder"
)

func BenchmarkHandlePacket(b *testing.B) {
	peeridA := newPeerID(b)
	peeridB := newPeerID(b)
	peeridC := newPeerID(b)

	fwd := forwarder.NewForwarder(log.New(ioutil.Discard, "", log.LstdFlags))
	fwd.Attach(peeridA, func(_ rovy.LowerPacket) error { return nil })
	fwd.Attach(peeridB, func(_ rovy.LowerPacket) error { return nil })
	fwd.Attach(peeridC, func(_ rovy.LowerPacket) error { return nil })

	upkt := rovy.NewUpperPacket(rovy.NewPacket(make([]byte, rovy.TptMTU)))
	upkt.SetRoute(rovy.NewRoute(0x2, 0x1))

	lpkt := rovy.NewLowerPacket(upkt.Packet)
	lpkt.LowerSrc = peeridA

	b.ReportAllocs()
	b.SetBytes(rovy.TptMTU)
	b.ResetTimer()

	var err error
	for i := 0; i < b.N; i++ {
		err = fwd.HandlePacket(lpkt)
		if err != nil {
			b.Fatalf("HandlePacket: %s", err)
		}
	}
}

func newPeerID(b *testing.B) rovy.PeerID {
	privkey, err := rovy.GeneratePrivateKey()
	if err != nil {
		b.Fatalf("NewPrivateKey: %s", err)
	}
	return rovy.NewPeerID(privkey.PublicKey())
}
