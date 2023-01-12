package fcnet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net/netip"

	dns "github.com/miekg/dns"
	ipv6 "golang.org/x/net/ipv6"
	wgnet "golang.zx2c4.com/wireguard/tun/netstack"

	rovy "go.rovy.net"
	node "go.rovy.net/node"
	forwarder "go.rovy.net/node/forwarder"
	rovyrt "go.rovy.net/node/routing"
)

const FcnetMulticodec = 0x42004
const PingMulticodec = 0x42005

const TunIfname = "rovy0"

var (
	linklocalPrefix = netip.MustParsePrefix("fe80::/64")
	multicastPrefix = netip.MustParsePrefix("ff00::/8")
	fc1Addr         = netip.MustParseAddr("fc00::1")
)

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
	LookupIPv6(netip.Addr) (rovy.PeerID, error)
}

type Fcnet struct {
	node    nodeIface
	routing routingIface
	log     *log.Logger
	ip      netip.Addr
	device  Device
	fc1net  *wgnet.Net
	fc1tun  Device
	fc1dns  *dns.Server
}

func NewFcnet(node nodeIface, dev Device) *Fcnet {
	fc := &Fcnet{
		node: node, ip: node.PeerID().PublicKey().IPAddr(), log: node.Log(), device: dev, routing: node.Routing(),
	}
	return fc
}

func (fc *Fcnet) Start(mtu int) error {
	fc.node.HandleLower(PingMulticodec, fc.handlePingPacket)
	fc.node.Handle(FcnetMulticodec, func(upkt rovy.UpperPacket) error {
		return fc.handleFcnetPacket(upkt.UpperSrc, upkt.Payload())
	})

	if err := fc.initFcnet1(mtu); err != nil {
		return err
	}

	if err := fc.initDns(); err != nil {
		return err
	}

	go fc.listenTun()

	return nil
}

func (fc *Fcnet) initFcnet1(mtu int) error {
	addrs := []netip.Addr{fc1Addr}
	dnssrv := []netip.Addr{}
	ftun, fnet, err := wgnet.CreateNetTUN(addrs, dnssrv, mtu)
	if err != nil {
		return err
	}
	fc.fc1tun = ftun
	fc.fc1net = fnet

	go func() {
		for {
			pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))
			buf := pkt.Bytes()[rovy.UpperOffset:]

			if _, err := fc.fc1tun.Read(buf, 0); err != nil {
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

func (fc *Fcnet) listenTun() {
	for {
		pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))
		buf := pkt.Bytes()[rovy.UpperOffset:]

		// TODO: "not pollable" error when device is deleted
		n, err := fc.device.Read(buf, 0)
		if err != nil {
			fc.log.Printf("fcnet: tun read: %s", err)
			continue
		}
		pkt.Length = rovy.UpperOffset + n

		if buf[0]>>4 != 6 {
			fc.log.Printf("tun: not an ipv6 packet: %#v buf=%#v", buf[0]>>4, buf[0:16])
			continue
		}

		if err := fc.handleTunPacket(buf[:n]); err != nil {
			fc.log.Printf("fcnet: handleTunPacket: %s", err)
			continue
		}
	}
}

func (fc *Fcnet) handleTunPacket(buf []byte) error {
	// fc.log.Printf("tun: got %#v", buf)

	plen := len(buf)

	// TODO: more checks
	if plen < ipv6.HeaderLen {
		return fmt.Errorf("tun: packet too short (len=%d)", plen)
	}

	gotlen := int(binary.BigEndian.Uint16(buf[4:6]))
	if plen != gotlen+ipv6.HeaderLen {
		return fmt.Errorf("fcnet: recv: length mismatch, expected %d, got %d (%d + %d)", plen, gotlen+ipv6.HeaderLen, gotlen, ipv6.HeaderLen)
	}

	nexthdr := buf[6]
	hops := int(buf[7])
	src, _ := netip.AddrFromSlice(buf[8:24])
	dst, _ := netip.AddrFromSlice(buf[24:40])

	if linklocalPrefix.Contains(src) || multicastPrefix.Contains(dst) {
		return nil
	}

	if src != fc.ip {
		fc.log.Printf("tun: dropping packet with illegal src address %s -> %s", src, dst)
		return nil
	}

	if dst == fc1Addr {
		// fc.log.Printf("tun: packet for fc00::1")
		_, err := fc.fc1tun.Write(buf, 0)
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
		upkt.SetCodec(FcnetMulticodec)
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
		ppkt.SetSender(ppkt.LowerSrc.PublicKey())
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

func (fc *Fcnet) handleFcnetPacket(src rovy.PeerID, payload []byte) error {
	n := len(payload)
	if n < ipv6.HeaderLen {
		return fmt.Errorf("fcnet: recv: packet too short (len=%d)", n)
	}
	gotlen := int(binary.BigEndian.Uint16(payload[4:6]))
	if n != gotlen+ipv6.HeaderLen {
		return fmt.Errorf("fcnet: recv: length mismatch, expected %d, got %d (%d + %d)", n, gotlen+ipv6.HeaderLen, gotlen, ipv6.HeaderLen)
	}
	if payload[0]>>4 != 0x6 {
		return fmt.Errorf("fcnet: recv: not an ipv6 packet")
	}
	if 0 != bytes.Compare(payload[8:24], src.PublicKey().IPAddr().AsSlice()) {
		return fmt.Errorf("fcnet: recv: src address mismatch")
	}
	if 0 != bytes.Compare(payload[24:40], fc.ip.AsSlice()) {
		return fmt.Errorf("fcnet: recv: dst address mismatch")
	}

	_, err := fc.device.Write(payload, 0)
	return err
}
