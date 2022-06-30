package node

import (
	"fmt"
	"log"
	"net"
	"net/netip"

	multiaddr "github.com/multiformats/go-multiaddr"
	rovy "go.rovy.net"
	ringbuf "go.rovy.net/util/ringbuf"
)

// tpt
// sess+mgram
// fwd|ping
// sess+mgram
// echo|fc00
// (rovyctl|fc00tun|mdns)

const TransportBufferSize = 1024

type Transport struct {
	conn       *net.UDPConn
	listenAddr rovy.Multiaddr
	sendQ      rovy.Queue
	logger     *log.Logger
}

func NewTransport(lisaddr rovy.Multiaddr, logger *log.Logger) (*Transport, error) {
	var network string
	protos := lisaddr.Protocols()
	if len(protos) != 2 || protos[1].Code != multiaddr.P_UDP {
		return nil, fmt.Errorf("can't listen on %s", lisaddr)
	}
	switch protos[0].Code {
	case multiaddr.P_IP6:
		network = "udp6"
	case multiaddr.P_IP4:
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

func (tpt *Transport) LocalMultiaddr() rovy.Multiaddr {
	ip := netip.MustParseAddrPort(tpt.conn.LocalAddr().String())
	return rovy.Multiaddr{IP: ip.Addr(), Port: ip.Port()}
}

func (tpt *Transport) RecvRoutine(recvQ rovy.Queue) {
	for {
		pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))

		n, raddr, err := tpt.conn.ReadFromUDPAddrPort(pkt.Bytes())
		if err != nil {
			tpt.logger.Printf("RecvRoutine: %s", err)
			continue
		}

		pkt.Length = n
		pkt.TptSrc = rovy.Multiaddr{IP: raddr.Addr(), Port: raddr.Port()}
		recvQ.Put(pkt)
	}
}

func (tpt *Transport) SendRoutine() {
	for {
		pkt := tpt.sendQ.Get()
		if pkt.TptDst.Empty() {
			tpt.logger.Printf("SendRoutine: dropping packet without TptSrc")
			continue
		}

		// tpt.logger.Printf("SendRoutine: writeTo: TptDst=%+v LowerDst=%+v UpperDst=%+v", pkt.TptDst, pkt.LowerDst, pkt.UpperDst)

		_, err := tpt.conn.WriteToUDPAddrPort(pkt.Bytes(), pkt.TptDst.AddrPort())
		if err != nil {
			tpt.logger.Printf("SendRoutine: %s", err)
		}
	}
}

func (tpt *Transport) Send(pkt rovy.Packet) {
	tpt.sendQ.PutWithBackpressure(pkt)
}
