package node

import (
	"log"

	multiaddr "github.com/multiformats/go-multiaddr"
	multiaddrnet "github.com/multiformats/go-multiaddr/net"
	rovy "pkt.dev/go-rovy"
	ringbuf "pkt.dev/go-rovy/util/ringbuf"
	// multigram "pkt.dev/go-rovy/multigram"
)

// tpt
// sess+mgram
// fwd|ping
// sess+mgram
// echo|fc00
// (rovyctl|fc00tun|mdns)

const TransportBufferSize = 1024

type Transport struct {
	conn   multiaddrnet.PacketConn
	sendQ  rovy.Queue
	logger *log.Logger
}

func NewTransport(lisaddr multiaddr.Multiaddr, logger *log.Logger) (*Transport, error) {
	pktconn, err := multiaddrnet.ListenPacket(lisaddr)
	if err != nil {
		return nil, err
	}

	tpt := &Transport{
		conn:   pktconn,
		sendQ:  ringbuf.NewRingBuffer(TransportBufferSize),
		logger: logger,
	}

	return tpt, nil
}

func (tpt *Transport) LocalMultiaddr() multiaddr.Multiaddr {
	return tpt.conn.LocalMultiaddr()
}

func (tpt *Transport) RecvRoutine(recvQ rovy.Queue) {
	for {
		pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))

		n, raddr, err := tpt.conn.ReadFrom(pkt.Bytes())
		if err != nil {
			tpt.logger.Printf("RecvRoutine: %s", err)
			continue
		}

		pkt.Length = n
		pkt.TptSrc, _ = multiaddrnet.FromNetAddr(raddr) // TODO handle error

		recvQ.Put(pkt)
	}
}

func (tpt *Transport) SendRoutine() {
	for {
		pkt := tpt.sendQ.Get()
		if pkt.TptDst != nil {
			_, err := tpt.conn.WriteToMultiaddr(pkt.Bytes(), pkt.TptDst)
			if err != nil {
				tpt.logger.Printf("SendRoutine: %s", err)
			}
		}
	}
}

func (tpt *Transport) Send(pkt rovy.Packet) {
	tpt.sendQ.Put(pkt)
}
