package fc00

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"

	icmp "golang.org/x/net/icmp"
	ipv6 "golang.org/x/net/ipv6"

	rovy "pkt.dev/go-rovy"
	node "pkt.dev/go-rovy/node"
)

const Fc00Multicodec = 0x42004
const PingMulticodec = 0x42005

type nodeIface interface {
	PeerID() rovy.PeerID
	Handle(uint64, node.DataHandler)
	SendUpper(rovy.PeerID, uint64, []byte, rovy.Route) error
	SendPlaintext(rovy.Route, uint64, []byte) error
	Log() *log.Logger
}

type routingIface interface {
	GetRoute(rovy.PeerID) (rovy.Route, error)
	LookupIPv6(net.IP) (rovy.PeerID, error)
}

type devIface interface {
	Read([]byte, int) (int, error)
	Write([]byte, int) (int, error)
	Close() error
}

const tunhdrOffset = 4

type Fc00 struct {
	node    nodeIface
	device  devIface
	routing routingIface
	log     *log.Logger
	cancel  func()
}

func NewFc00(node nodeIface, dev devIface, routing routingIface) *Fc00 {
	fc := &Fc00{
		node: node, log: node.Log(), device: dev, routing: routing, cancel: func() {},
	}
	return fc
}

func (fc *Fc00) Start() error {
	fc.node.Handle(Fc00Multicodec, fc.handleFc00Packet)
	fc.node.Handle(PingMulticodec, fc.handlePingPacket)

	go func() {
		zeros := []byte{0x0, 0x0, 0x0, 0x0}

		for {
			pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))
			buf := pkt.Bytes()[rovy.UpperOffset:]

			n, err := fc.device.Read(buf, 4)
			if err != nil {
				fc.log.Printf("fc00: tun read: %s", err)
				continue
			}
			pkt.Length = rovy.UpperOffset + 4 + n

			if 0 != bytes.Compare([]byte{0x86, 0xdd}, buf[2:4]) {
				fc.log.Printf("tun: not an ipv6 packet")
				continue
			}
			copy(buf[0:4], zeros)

			if err := fc.handleTunPacket(buf[4 : 4+n]); err != nil {
				fc.log.Printf("fc00: handleTunPacket: %s", err)
				continue
			}
		}
	}()

	return nil
}

func (fc *Fc00) Stop() error {
	// fc.node.Detach(Fc00Multicodec)
	// fc.node.Detach(PingMulticodec)
	// fc.device.Close()
	// fc.cancel()
	return nil
}

func (fc *Fc00) handleTunPacket(buf []byte) error {
	// fc.log.Printf("tun: got %#v", buf)

	plen := len(buf)

	// TODO: more checks
	if plen < ipv6.HeaderLen {
		return fmt.Errorf("tun: packet too short (len=%d)", plen)
	}

	gotlen := int(binary.BigEndian.Uint16(buf[4:6]))
	if plen != gotlen+ipv6.HeaderLen {
		return fmt.Errorf("fc00: recv: length mismatch, expected %d, got %d (%d + %d)", plen, gotlen+ipv6.HeaderLen, gotlen, ipv6.HeaderLen)
	}

	nexthdr := buf[6]
	hops := int(buf[7])
	src := net.IP(buf[8:24])
	dst := net.IP(buf[24:40])

	// no link-local or multicast stuff yet
	if dst.IsMulticast() || !src.Equal(fc.node.PeerID().PublicKey().Addr()) {
		// fc.log.Printf("tun: dropping multicast packet %s -> %s", src, dst)
		return nil
	}

	// fc.log.Printf("tun: buflen=%d hdrlen=%d+%d", plen, gotlen, ipv6.HeaderLen)

	peerid, err := fc.routing.LookupIPv6(dst)
	if err != nil {
		return err
	}

	route, err := fc.routing.GetRoute(peerid)
	if err != nil {
		return fmt.Errorf("tun: no route for %s: %s", peerid, err)
	}

	buf = buf[:plen]

	// end-to-end transmission
	if hops >= route.Len() {
		return fc.node.SendUpper(peerid, Fc00Multicodec, buf, route)
	}

	// icmp with ttl that'd exceed in transit
	if nexthdr == byte(58) {
		rt := rovy.NewRoute(route.Bytes()[0:hops]...)
		return fc.node.SendPlaintext(rt, PingMulticodec, buf)
	}

	fc.log.Printf("tun: fc00 packet %s -> %s dropped (ttl=%d, nexthdr=%d)", src, dst, hops, nexthdr)

	return nil
}

func (fc *Fc00) handleFc00Packet(buf []byte, pid rovy.PeerID, rt rovy.Route) error {
	n := len(buf)
	if n < ipv6.HeaderLen {
		fc.log.Printf("fc00: recv: packet too short (len=%d)", n)
		return nil
	}
	gotlen := int(binary.BigEndian.Uint16(buf[4:6]))
	if n != gotlen+ipv6.HeaderLen {
		fc.log.Printf("fc00: recv: length mismatch, expected %d, got %d (%d + %d)", n, gotlen+ipv6.HeaderLen, gotlen, ipv6.HeaderLen)
		// TODO send icmp errors
		return nil
	}
	if buf[0]>>4 != 0x6 {
		fc.log.Printf("fc00: recv: not an ipv6 packet")
		return nil
	}
	if 0 != bytes.Compare(buf[8:24], pid.PublicKey().Addr()) {
		fc.log.Printf("fc00: recv: src address mismatch")
		return nil
	}
	if 0 != bytes.Compare(buf[24:40], fc.node.PeerID().PublicKey().Addr()) {
		fc.log.Printf("fc00: recv: dst address mismatch")
		return nil
	}

	buf = append([]byte{0x0, 0x0, 0x86, 0xdd}, buf...) // XXX slowness

	_, err := fc.device.Write(buf, tunhdrOffset)
	return err
}

func (fc *Fc00) handlePingPacket(buf []byte, pid rovy.PeerID, rt rovy.Route) error {
	if buf[40] == byte(ipv6.ICMPTypeEchoRequest) {
		// fc.log.Printf("fc00: icmp-echo-request from %s @ %s", peerid, route)

		src := pid.PublicKey().Addr()
		if 0 != bytes.Compare(buf[8:24], src) {
			fc.log.Printf("fc00: recv: src address mismatch")
			return nil
		}

		src2 := fc.node.PeerID().PublicKey().Addr()
		dst2 := src

		body := &icmp.TimeExceeded{Data: buf[:]}
		msg := icmp.Message{
			Type: ipv6.ICMPTypeTimeExceeded,
			Code: 0,
			Body: body,
		}

		icmpdata, err := msg.Marshal(icmp.IPv6PseudoHeader(src2, dst2))
		if err != nil {
			fc.log.Printf("fc00: icmp packet construction error: %s", err)
			return err
		}

		// TODO: make sure the resulting packet doesn't exceed MTU
		ilen := len(icmpdata)
		p2 := make([]byte, 40+ilen)
		copy(p2[0:4], buf[0:4])
		binary.BigEndian.PutUint16(p2[4:6], uint16(ilen))
		p2[6] = 58
		p2[7] = 255
		copy(p2[8:24], src2)
		copy(p2[24:40], dst2)
		copy(p2[40:], icmpdata)

		return fc.node.SendPlaintext(rt, PingMulticodec, p2)
	}

	if buf[40] == byte(ipv6.ICMPTypeTimeExceeded) {
		// fc.log.Printf("fc00: icmp-time-exceeded from %s @ %s", peerid, route)

		src := pid.PublicKey().Addr()
		if 0 != bytes.Compare(buf[8:24], src) {
			fc.log.Printf("fc00: recv: src address mismatch")
			return nil
		}

		dst := fc.node.PeerID().PublicKey().Addr()
		if 0 != bytes.Compare(buf[24:40], dst) {
			fc.log.Printf("fc00: recv: dst address mismatch")
			return nil
		}

		return fc.handleFc00Packet(buf, pid, rt)
	}

	// rnode.Log().Printf("fc00: dropping ping packet from %s @ %s %#v", peerid, route, p)
	return nil
}
