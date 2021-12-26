package fc00

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"

	icmp "golang.org/x/net/icmp"
	ipv6 "golang.org/x/net/ipv6"

	rovy "github.com/rovynet/go-rovy"
	forwarder "github.com/rovynet/go-rovy/forwarder"
	node "github.com/rovynet/go-rovy/node"
	routing "github.com/rovynet/go-rovy/routing"
)

const Fc00Multicodec = 0x42004
const PingMulticodec = 0x42005

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
	Close() error
}

const tunhdrOffset = 4

type Fc00 struct {
	node    nodeIface
	device  devIface
	routing routingIface
	log     *log.Logger
}

func NewFc00(node nodeIface, dev devIface, routing routingIface) *Fc00 {
	fc := &Fc00{
		node: node, log: node.Log(), device: dev, routing: routing,
	}
	return fc
}

func (fc *Fc00) Start() error {
	fc.node.HandleLower(PingMulticodec, fc.handlePingPacket)
	fc.node.Handle(Fc00Multicodec, func(upkt rovy.UpperPacket) error {
		return fc.handleFc00Packet(upkt.UpperSrc, upkt.Payload())
	})

	go fc.listenTun()

	return nil
}

func (fc *Fc00) handlePingPacket(lpkt rovy.LowerPacket) error {
	ppkt := NewPingPacket(lpkt.Packet)

	if !ppkt.IsDestination() {
		// fc.log.Printf("handlePingPacket: forwarding")
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

	// fc.log.Printf("handlePingPacket: lowerSrc=%s fwdhdr=%#v revroute=%s reqid=%#v", lpkt.LowerSrc, fwdhdr, route, ppkt.RequestId())

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

		icmpdata, err := msg.Marshal(icmp.IPv6PseudoHeader(src2, dst2))
		if err != nil {
			return fmt.Errorf("fc00: icmp packet construction error: %s", err)
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

// TODO implement me
func (fc *Fc00) sign(ppkt PingPacket) (PingPacket, error) {
	return ppkt, nil
}

// TODO implement me
func (fc *Fc00) verify(ppkt PingPacket) error {
	return nil
}

func (fc *Fc00) listenTun() {
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
		pkt.Buf = pkt.Buf[4:]

		if err := fc.handleTunPacket(buf[4 : 4+n]); err != nil {
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
	if dst.IsMulticast() || !src.Equal(fc.node.PeerID().PublicKey().Addr()) {
		// fc.log.Printf("tun: dropping multicast packet %s -> %s", src, dst)
		return nil
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

	fc.log.Printf("tun: fc00 packet %s -> %s dropped (ttl=%d, nexthdr=%d)", src, dst, hops, nexthdr)

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

	payload = append([]byte{0x0, 0x0, 0x86, 0xdd}, payload...) // XXX slowness

	_, err := fc.device.Write(payload, tunhdrOffset)
	return err
}
