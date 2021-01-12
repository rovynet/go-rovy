package forwarder_test

import (
	"io/ioutil"
	"log"
	"testing"

	// forwarder "go.rovy.net/forwarder"
	rovy "pkt.dev/go-rovy"
	forwarder "pkt.dev/go-rovy/forwarder"
)

func BenchmarkHandlePacket(b *testing.B) {
	peeridA := newPeerID(b)
	peeridB := newPeerID(b)
	peeridC := newPeerID(b)

	fwd := forwarder.NewForwarder(16, log.New(ioutil.Discard, "", log.LstdFlags))
	fwd.Attach(peeridA, func(_ rovy.PeerID, _ []byte) error { return nil })
	fwd.Attach(peeridB, func(_ rovy.PeerID, _ []byte) error { return nil })
	fwd.Attach(peeridC, func(_ rovy.PeerID, _ []byte) error { return nil })

	buf := make([]byte, 1400)
	pkt := []byte{0x0, 0x2, 0x2, 0x1, 0x0, 0x0, 0x0, 0x0}
	var err error

	b.ReportAllocs()
	b.SetBytes(1400)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(buf, pkt)
		err = fwd.HandlePacket(buf, peeridA)
		if err != nil {
			b.Fatalf("HandlePacket: %s", err)
		}
	}
}

func newPeerID(b *testing.B) rovy.PeerID {
	privkey, err := rovy.NewPrivateKey()
	if err != nil {
		b.Fatalf("NewPrivateKey: %s", err)
	}
	return rovy.NewPeerID(privkey.PublicKey())
}
