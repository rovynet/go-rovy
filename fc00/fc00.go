package rovyfc00

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/netip"

	dns "github.com/miekg/dns"
	ipv6 "golang.org/x/net/ipv6"

	rovy "go.rovy.net"
	rovygvtun "go.rovy.net/fc00/gvisor"
	forwarder "go.rovy.net/forwarder"
	node "go.rovy.net/node"
	rovyrt "go.rovy.net/routing"
)

const Fc00Multicodec = 0x42004
const PingMulticodec = 0x42005

const TunIfname = "rovy0"

type nodeIface interface {
	PeerID() rovy.PeerID
	Handle(uint64, node.UpperHandler)
	HandleLower(uint64, node.LowerHandler)
	Forwarder() *forwarder.Forwarder
	Routing() *rovyrt.Routing
	SendUpper(rovy.UpperPacket) error
	Log() *log.Logger
}

type routingIface interface {
	GetRoute(rovy.PeerID) (rovy.Route, error)
	LookupIPv6(net.IP) (rovy.PeerID, error)
}

type Fc00 struct {
	node     nodeIface
	device   Device
	routing  routingIface
	log      *log.Logger
	fc001net rovygvtun.GvisorNet
	fc001tun Device
	fc001dns *dns.Server
}

func NewFc00(node nodeIface, dev Device) *Fc00 {
	fc := &Fc00{
		node: node, log: node.Log(), device: dev, routing: node.Routing(),
	}
	return fc
}

func (fc *Fc00) Start(mtu int) error {
	fc.node.HandleLower(PingMulticodec, fc.handlePingPacket)
	fc.node.Handle(Fc00Multicodec, func(upkt rovy.UpperPacket) error {
		return fc.handleFc00Packet(upkt.UpperSrc, upkt.Payload())
	})

	go fc.listenTun()

	if err := fc.initFc001(mtu); err != nil {
		return err
	}

	if err := fc.initDns(); err != nil {
		return err
	}

	return nil
}

func (fc *Fc00) initFc001(mtu int) error {
	ftun, fnet, err := rovygvtun.NewGvisorTUN(netip.MustParseAddr("fc00::1"), mtu, fc.log)
	if err != nil {
		return err
	}
	fc.fc001tun = ftun
	fc.fc001net = fnet

	go func() {
		for {
			pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))
			buf := pkt.Bytes()[rovy.UpperOffset:]

			if _, err := fc.fc001tun.Read(buf, 0); err != nil {
				fc.log.Printf("dns: tun read: %s", err)
				continue
			}

			if _, err = fc.device.Write(buf, 0); err != nil {
				fc.log.Printf("dns: tun write: %s", err)
				continue
			}
		}
	}()

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
		_, err := fc.fc001tun.Write(buf, 0)
		return err
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
