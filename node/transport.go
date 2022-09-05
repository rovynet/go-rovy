package node

import (
	"fmt"
	"log"
	"net"
	"net/netip"

	rovy "go.rovy.net"
	ringbuf "go.rovy.net/node/util/ringbuf"
)

// tpt
// sess+mgram
// fwd|ping
// sess+mgram
// echo|fcnet
// (rovyctl|fcnettun|mdns)

const TransportBufferSize = 1024

type Transport struct {
	conn       *net.UDPConn
	listenAddr rovy.Multiaddr
	running    chan int
	sendQ      *ringbuf.RingBuffer
	logger     *log.Logger
}

func NewTransport(lisaddr rovy.Multiaddr, logger *log.Logger) (*Transport, error) {
	var network string
	protos := lisaddr.Protocols()
	if len(protos) != 2 || protos[1].Code != rovy.UDPMultiaddrCodec {
		return nil, fmt.Errorf("can't listen on %s", lisaddr)
	}
	switch protos[0].Code {
	case rovy.IP6MultiaddrCodec:
		network = "udp6"
	case rovy.IP4MultiaddrCodec:
		network = "udp4"
	default:
		return nil, fmt.Errorf("can't listen on %s", lisaddr)
	}

	udpaddr := net.UDPAddrFromAddrPort(lisaddr.AddrPort())
	conn, err := net.ListenUDP(network, udpaddr)
	if err != nil {
		return nil, err
	}

	tpt := &Transport{
		conn:       conn,
		listenAddr: lisaddr,
		sendQ:      ringbuf.NewRingBuffer(TransportBufferSize),
		logger:     logger,
	}

	return tpt, nil
}

func (tpt *Transport) Start(next *ringbuf.RingBuffer) error {
	if tpt.Running() {
		return ErrRunning
	}

	tpt.running = make(chan int)
	go tpt.SendRoutine()
	go tpt.RecvRoutine(next)

	return nil
}

func (tpt *Transport) Stop() error {
	if !tpt.Running() {
		return ErrNotRunning
	}

	close(tpt.running)

	return nil
}

func (tpt *Transport) Running() bool {
	if tpt.running != nil {
		select {
		case <-tpt.running:
			// we're not running anymore, channel is closed.
			// the channel is unbuffered, so if we never write anything to it,
			// then the only way to reach here is if the channel is closed.
			return false
		default:
			return true
		}
	}
	return false
}

func (tpt *Transport) LocalMultiaddr() rovy.Multiaddr {
	ip := netip.MustParseAddrPort(tpt.conn.LocalAddr().String())
	return rovy.Multiaddr{IP: ip.Addr(), Port: ip.Port()}
}

func (tpt *Transport) RecvRoutine(next *ringbuf.RingBuffer) {
	for {
		pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))

		n, raddr, err := tpt.conn.ReadFromUDPAddrPort(pkt.Bytes())
		if err == net.ErrClosed {
			tpt.logger.Printf("closing transport recv routine, socket is closed")
			return
		}
		if err != nil {
			tpt.logger.Printf("RecvRoutine: %s", err)
			continue
		}

		pkt.Length = n
		pkt.TptSrc = rovy.Multiaddr{IP: raddr.Addr(), Port: raddr.Port()}
		next.Put(pkt)
	}
}

func (tpt *Transport) SendRoutine() {
	for {
		select {
		case <-tpt.running:
			return
		case pkt := <-tpt.sendQ.Channel():
			if pkt.TptDst.Empty() {
				tpt.logger.Printf("SendRoutine: dropping packet without TptSrc")
				continue
			}

			// tpt.logger.Printf("SendRoutine: writeTo: TptDst=%+v LowerDst=%+v UpperDst=%+v", pkt.TptDst, pkt.LowerDst, pkt.UpperDst)

			_, err := tpt.conn.WriteToUDPAddrPort(pkt.Bytes(), pkt.TptDst.AddrPort())
			if err == net.ErrClosed {
				tpt.logger.Printf("closing transport send routine, socket is closed")
				return
			}
			if err != nil {
				tpt.logger.Printf("SendRoutine: %s", err)
			}
		}
	}
}

func (tpt *Transport) Send(pkt rovy.Packet) error {
	tpt.sendQ.PutWithBackpressure(pkt)
	return nil
}
