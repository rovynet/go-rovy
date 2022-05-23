package fc00

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"

	dns "github.com/miekg/dns"
	icmp "golang.org/x/net/icmp"
	ipv6 "golang.org/x/net/ipv6"

	rovy "go.rovy.net"
	forwarder "go.rovy.net/forwarder"
	node "go.rovy.net/node"
	routing "go.rovy.net/routing"
)

const Fc00Multicodec = 0x42004
const PingMulticodec = 0x42005

const TunIfname = "rovy0"

type nodeIface interface {
	PeerID() rovy.PeerID
	Handle(uint64, node.UpperHandler)
	HandleLower(uint64, node.LowerHandler)
	Forwarder() *forwarder.Forwarder
	Routing() *routing.Routing
	SendUpper(rovy.UpperPacket) error
	Log() *log.Logger
}

type routingIface interface {
	GetRoute(rovy.PeerID) (rovy.Route, error)
	LookupIPv6(net.IP) (rovy.PeerID, error)
}

type devIface interface {
	Read([]byte, int) (int, error)
	Write([]byte, int) (int, error)
	MTU() (int, error)
	Close() error
}

type Fc00 struct {
	node     nodeIface
	device   devIface
	routing  routingIface
	log      *log.Logger
	fc001tun devIface
	fc001dns *dns.Server
}

func NewFc00(node nodeIface, dev devIface, routing routingIface) *Fc00 {
	fc := &Fc00{
		node: node, log: node.Log(), device: dev, routing: routing,
	}
	return fc
}

func (fc *Fc00) Start(mtu int, initDns bool) error {
	fc.node.HandleLower(PingMulticodec, fc.handlePingPacket)
	fc.node.Handle(Fc00Multicodec, func(upkt rovy.UpperPacket) error {
		return fc.handleFc00Packet(upkt.UpperSrc, upkt.Payload())
	})

	go fc.listenTun()

	if initDns {
		if err := fc.initDns(fc.node.PeerID(), mtu); err != nil {
			return err
		}
	}

	return nil
}

func (fc *Fc00) handlePingPacket(lpkt rovy.LowerPacket) error {
	ppkt := NewPingPacket(lpkt.Packet)

	if !ppkt.IsDestination() {
		return fc.node.Forwarder().HandlePacket(lpkt)
	}

	prevrt, err := fc.node.Routing().GetRoute(lpkt.LowerSrc)
	if err != nil {
		return err
	}
	fwdhdr := ppkt.Buf[ppkt.Offset+4 : ppkt.Offset+20]
	fwdhdr[2+fwdhdr[0]] = prevrt.Bytes()[0]
	fwdhdr[0] = fwdhdr[0] + 1

	route := rovy.NewRoute(fwdhdr[2 : 2+fwdhdr[0]]...).Reverse()

	if ppkt.IsReply() {
		if err := fc.verify(ppkt); err != nil {
			return err
		}

		buf := ppkt.Payload()

		dst := fc.node.PeerID().PublicKey().Addr()
		if 0 != bytes.Compare(buf[8:24], dst) {
			return fmt.Errorf("fc00: recv: dst address mismatch -- expected %#v -- got %#v", dst, buf[8:24])
		}

		src2 := ppkt.Sender().Addr()
		dst2 := dst

		body := &icmp.TimeExceeded{Data: buf[:]}
		msg := icmp.Message{
			Type: ipv6.ICMPTypeTimeExceeded,
			Code: 0,
			Body: body,
		}

		// about traceroute on fedora:
		//
		// firewalld drops these replies although they're fine.
		// it probably recognizes the hops as local interfaces
		// and partially blocks incoming cross-interface icmp.
		//
		// to verify, try traceroute non-local peerings: laptop->desktop->server
		// or temporarily shutdown firewalld.
		//
		chdr := icmp.IPv6PseudoHeader(src2, dst2)
		cbuf := append(chdr, 0x3, 0x0, 0, 0)
		cbody, err := msg.Body.Marshal(ipv6.ICMPTypeTimeExceeded.Protocol())
		if err != nil {
			return fmt.Errorf("fc00: icmp checksuming error: %s", err)
		}
		cbuf = append(cbuf, cbody...)
		lenoff := 2 * net.IPv6len
		binary.BigEndian.PutUint32(cbuf[lenoff:lenoff+4], uint32(len(cbuf)-len(chdr)))
		csum := icmpChecksum(cbuf)
		cbuf[len(chdr)+2] ^= byte(csum)
		cbuf[len(chdr)+3] ^= byte(csum >> 8)
		icmpdata := cbuf[len(chdr):]

		// TODO: make sure the resulting packet doesn't exceed MTU
		ilen := len(icmpdata)
		p2 := make([]byte, 40+ilen)
		copy(p2[0:4], buf[0:4]) // copying the src flowlabel might be stupid
		binary.BigEndian.PutUint16(p2[4:6], uint16(ilen))
		p2[6] = 58
		p2[7] = 64
		copy(p2[8:24], src2)
		copy(p2[24:40], dst2)
		copy(p2[40:], icmpdata)

		return fc.handleFc00Packet(rovy.NewPeerID(ppkt.Sender()), p2)
	}

	ppkt2 := NewPingPacket(rovy.NewPacket(make([]byte, rovy.TptMTU)))
	ppkt2.LowerSrc = fc.node.PeerID()
	ppkt2.SetRoute(route)
	ppkt2.SetSender(fc.node.PeerID().PublicKey())
	ppkt2.SetIsReply(true)
	ppkt2 = ppkt2.SetPayload(ppkt.Payload())

	ppkt2, err = fc.sign(ppkt2)
	if err != nil {
		return err
	}

	lpkt2 := rovy.NewLowerPacket(ppkt2.Packet)
	lpkt2.SetCodec(PingMulticodec)
	return fc.node.Forwarder().SendRaw(lpkt2)
}

// From golang.org/x/net/icmp
func icmpChecksum(b []byte) uint16 {
	csumcv := len(b) - 1 // checksum coverage
	s := uint32(0)
	for i := 0; i < csumcv; i += 2 {
		s += uint32(b[i+1])<<8 | uint32(b[i])
	}
	if csumcv&1 == 0 {
		s += uint32(b[csumcv])
	}
	s = s>>16 + s&0xffff
	s = s + s>>16
	return ^uint16(s)
}

// TODO implement me
func (fc *Fc00) sign(ppkt PingPacket) (PingPacket, error) {
	return ppkt, nil
}

// TODO implement me
func (fc *Fc00) verify(ppkt PingPacket) error {
	return nil
}

func (fc *Fc00) listenTun() {
	for {
		pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))
		buf := pkt.Bytes()[rovy.UpperOffset:]

		n, err := fc.device.Read(buf, 0)
		if err != nil {
			fc.log.Printf("fc00: tun read: %s", err)
			continue
		}
		pkt.Length = rovy.UpperOffset + n

		if buf[0]>>4 != 6 {
			fc.log.Printf("tun: not an ipv6 packet: %#v buf=%#v", buf[0]>>4, buf[0:16])
			continue
		}

		if err := fc.handleTunPacket(buf[:n]); err != nil {
			fc.log.Printf("fc00: handleTunPacket: %s", err)
			continue
		}
	}
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
	if src[0] == 0xfe || dst[0] == 0xff {
		// fc.log.Printf("tun: dropping packet %s -> %s", src, dst)
		return nil
	}

	if !src.Equal(fc.node.PeerID().PublicKey().Addr()) {
		fc.log.Printf("tun: dropping packet with illegal src address %s -> %s", src, dst)
		return nil
	}

	if dst.Equal(net.ParseIP("fc00::1")) {
		// fc.log.Printf("tun: packet for fc00::1")
		return fc.handleDnsRequest(buf)
	}

	peerid, err := fc.routing.LookupIPv6(dst)
	if err != nil {
		return err
	}

	route, err := fc.routing.GetRoute(peerid)
	if err != nil {
		return fmt.Errorf("tun: no route for %s: %s", peerid, err)
	}

	// end-to-end transmission
	if hops >= route.Len() {
		upkt := rovy.NewUpperPacket(rovy.NewPacket(make([]byte, rovy.TptMTU)))
		upkt.UpperDst = peerid
		upkt.SetRoute(route)
		upkt.SetCodec(Fc00Multicodec)
		upkt = upkt.SetPayload(buf)
		return fc.node.SendUpper(upkt)
	}

	// icmp with ttl that'd exceed in transit
	if nexthdr == byte(58) {
		r := route.Bytes()
		if len(r) > hops {
			r = r[:hops]
		}
		rt := rovy.NewRoute(r...)

		ppkt := NewPingPacket(rovy.NewPacket(make([]byte, rovy.TptMTU)))
		ppkt.LowerSrc = fc.node.PeerID()
		ppkt.SetRoute(rt)
		ppkt.SetSender(fc.node.PeerID().PublicKey())
		ppkt.SetIsReply(false)
		ppkt = ppkt.SetPayload(buf)

		ppkt, err := fc.sign(ppkt)
		if err != nil {
			return err
		}

		lpkt := rovy.NewLowerPacket(ppkt.Packet)
		lpkt.SetCodec(PingMulticodec)

		return fc.node.Forwarder().SendRaw(lpkt)
	}

	fc.log.Printf(
		"tun: dropped outgoing %s -> %s - ttl is too low for non-icmp (nexthdr=%d)",
		src, dst, nexthdr)

	return nil
}

func (fc *Fc00) handleFc00Packet(src rovy.PeerID, payload []byte) error {
	n := len(payload)
	if n < ipv6.HeaderLen {
		return fmt.Errorf("fc00: recv: packet too short (len=%d)", n)
	}
	gotlen := int(binary.BigEndian.Uint16(payload[4:6]))
	if n != gotlen+ipv6.HeaderLen {
		return fmt.Errorf("fc00: recv: length mismatch, expected %d, got %d (%d + %d)", n, gotlen+ipv6.HeaderLen, gotlen, ipv6.HeaderLen)
	}
	if payload[0]>>4 != 0x6 {
		return fmt.Errorf("fc00: recv: not an ipv6 packet")
	}
	if 0 != bytes.Compare(payload[8:24], src.PublicKey().Addr()) {
		return fmt.Errorf("fc00: recv: src address mismatch")
	}
	if 0 != bytes.Compare(payload[24:40], fc.node.PeerID().PublicKey().Addr()) {
		return fmt.Errorf("fc00: recv: dst address mismatch")
	}

	_, err := fc.device.Write(payload, 0)
	return err
}
