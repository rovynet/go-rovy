package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"golang.org/x/net/icmp"
	ipv6 "golang.org/x/net/ipv6"

	multiaddr "github.com/multiformats/go-multiaddr"
	netlink "github.com/vishvananda/netlink"
	netns "github.com/vishvananda/netns"
	tun "golang.zx2c4.com/wireguard/tun"

	rovy "pkt.dev/go-rovy"
	node "pkt.dev/go-rovy/node"
)

const Fc00Multicodec = 0x42004
const PingMulticodec = 0x42005

var addrbook = map[string]rovy.PeerID{}

func newNode(name string, lisaddr multiaddr.Multiaddr) (*node.Node, error) {
	logger := log.New(os.Stderr, "["+name+"] ", log.Ltime|log.Lshortfile)

	privkey, err := rovy.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	node := node.NewNode(privkey, logger)

	if err = node.Listen(lisaddr); err != nil {
		return nil, err
	}

	addrbook[node.PeerID().PublicKey().Addr().String()] = node.PeerID()

	logger.Printf("%s/rovy/%s", lisaddr, node.PeerID())
	logger.Printf("/ip6/%s", node.PeerID().PublicKey().Addr())
	return node, nil
}

func newTun(ifname string, ip6 net.IP, mtu int, logger *log.Logger) (tun.Device, error) {
	dev, err := tun.CreateTUN(ifname, mtu)
	if err != nil {
		return nil, err
	}

	ifname2, err := dev.Name()
	if err != nil {
		dev.Close()
		return nil, err
	}

	link, err := netlink.LinkByName(ifname2)
	if err != nil {
		dev.Close()
		return nil, err
	}

	nladdr, err := netlink.ParseAddr(ip6.String() + "/128")
	if err != nil {
		dev.Close()
		return nil, err
	}

	err = netlink.AddrAdd(link, nladdr)
	if err != nil {
		dev.Close()
		return nil, err
	}

	err = netlink.LinkSetUp(link)
	if err != nil {
		dev.Close()
		return nil, err
	}

	_, ip6cidr, err := net.ParseCIDR("fc00::/8")
	if err != nil {
		dev.Close()
		return nil, err
	}

	err = netlink.RouteAdd(&netlink.Route{LinkIndex: link.Attrs().Index, Dst: ip6cidr})
	if err != nil {
		logger.Printf("fc00: failed to add route %s => %s, skipping", ip6cidr, ifname)
	} else {
		logger.Printf("fc00: added route %s => %s", ip6cidr, ifname)
	}

	return dev, nil
}

func newTunNamespace(ifname string, ip6 net.IP, mtu int, logger *log.Logger) (tun.Device, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origns, err := netns.Get()
	if err != nil {
		return nil, err
	}
	defer origns.Close()

	newns, err := netns.New()
	if err != nil {
		return nil, err
	}
	defer newns.Close()

	dev, err := newTun(ifname, ip6, mtu, logger)
	netns.Set(origns)
	return dev, err
}

func run() error {
	addrA := multiaddr.StringCast("/ip4/127.0.0.1/udp/12345")
	addrB := multiaddr.StringCast("/ip4/127.0.0.1/udp/12346")
	addrC := multiaddr.StringCast("/ip4/127.0.0.1/udp/12347")
	addrD := multiaddr.StringCast("/ip4/127.0.0.1/udp/12348")

	nodeA, err := newNode("nodeA", addrA)
	if err != nil {
		return err
	}
	nodeB, err := newNode("nodeB", addrB)
	if err != nil {
		return err
	}
	nodeC, err := newNode("nodeC", addrC)
	if err != nil {
		return err
	}
	nodeD, err := newNode("nodeD", addrD)
	if err != nil {
		return err
	}

	devA, err := newTun("rovy0", nodeA.PeerID().PublicKey().Addr(), rovy.PreliminaryMTU, nodeA.Log())
	if err != nil {
		return err
	}

	devB, err := newTun("rovy1", nodeB.PeerID().PublicKey().Addr(), rovy.PreliminaryMTU, nodeB.Log())
	if err != nil {
		return err
	}

	devC, err := newTun("rovy2", nodeC.PeerID().PublicKey().Addr(), rovy.PreliminaryMTU, nodeC.Log())
	if err != nil {
		return err
	}

	devD, err := newTunNamespace("rovy3", nodeD.PeerID().PublicKey().Addr(), rovy.PreliminaryMTU, nodeD.Log())
	if err != nil {
		return err
	}

	// ordering is important, need to register multicodec before establishing any sessions
	go doTheThing(nodeA, devA)
	go doTheThing(nodeB, devB)
	go doTheThing(nodeC, devC)
	go doTheThing(nodeD, devD)

	if err := nodeA.Connect(nodeB.PeerID(), addrB); err != nil {
		nodeA.Log().Printf("failed to connect nodeA to nodeB: %s", err)
		return err
	}
	if err := nodeB.Connect(nodeC.PeerID(), addrC); err != nil {
		nodeB.Log().Printf("failed to connect nodeB to nodeC: %s", err)
		return err
	}
	if err := nodeC.Connect(nodeD.PeerID(), addrD); err != nil {
		nodeC.Log().Printf("failed to connect nodeC to nodeD: %s", err)
		return err
	}

	// A->B->C->D
	nodeA.Routing().AddRoute(nodeD.PeerID(),
		nodeA.Routing().MustGetRoute(nodeB.PeerID()).
			Join(nodeB.Routing().MustGetRoute(nodeC.PeerID())).
			Join(nodeC.Routing().MustGetRoute(nodeD.PeerID())))
	if err := nodeA.Connect(nodeD.PeerID(), nil); err != nil {
		nodeA.Log().Printf("failed to connect nodeA to nodeD: %s", err)
		return err
	}

	nodeA.Routing().PrintTable(nodeA.Log())

	select {}
	return nil
}

const tunhdrOffset = 4

func consumeIncomingPacket(rnode *node.Node, dev tun.Device, p []byte, peerid rovy.PeerID, route rovy.Route) error {
	n := len(p)
	if n < ipv6.HeaderLen {
		rnode.Log().Printf("fc00: recv: packet too short (len=%d)", n)
		return nil
	}
	gotlen := int(binary.BigEndian.Uint16(p[4:6]))
	if n != gotlen+ipv6.HeaderLen {
		rnode.Log().Printf("fc00: recv: length mismatch, expected %d, got %d (%d + %d)", n, gotlen+ipv6.HeaderLen, gotlen, ipv6.HeaderLen)
		// TODO send icmp errors
		return nil
	}
	if p[0]>>4 != 0x6 {
		rnode.Log().Printf("fc00: recv: not an ipv6 packet")
		return nil
	}
	if 0 != bytes.Compare(p[8:24], peerid.PublicKey().Addr()) {
		rnode.Log().Printf("fc00: recv: src address mismatch")
		return nil
	}
	if 0 != bytes.Compare(p[24:40], rnode.PeerID().PublicKey().Addr()) {
		rnode.Log().Printf("fc00: recv: dst address mismatch")
		return nil
	}

	p = append([]byte{0x0, 0x0, 0x86, 0xdd}, p...) // XXX slowness

	_, err := dev.Write(p, tunhdrOffset)
	return err
}

func doTheThing(rnode *node.Node, dev tun.Device) error {
	rnode.Handle(Fc00Multicodec, func(p []byte, peerid rovy.PeerID, route rovy.Route) error {
		return consumeIncomingPacket(rnode, dev, p, peerid, route)
	})

	rnode.Handle(PingMulticodec, func(p []byte, peerid rovy.PeerID, route rovy.Route) error {
		if p[40] == byte(ipv6.ICMPTypeEchoRequest) {
			// rnode.Log().Printf("fc00: icmp-echo-request from %s @ %s", peerid, route)

			src := peerid.PublicKey().Addr()
			if 0 != bytes.Compare(p[8:24], src) {
				rnode.Log().Printf("fc00: recv: src address mismatch")
				return nil
			}

			src2 := rnode.PeerID().PublicKey().Addr()
			dst2 := src

			body := &icmp.TimeExceeded{Data: p[:]}
			msg := icmp.Message{
				Type: ipv6.ICMPTypeTimeExceeded,
				Code: 0,
				Body: body,
			}

			icmpdata, err := msg.Marshal(icmp.IPv6PseudoHeader(src2, dst2))
			if err != nil {
				rnode.Log().Printf("fc00: icmp packet construction error: %s", err)
				return err
			}

			// TODO: make sure the resulting packet doesn't exceed MTU
			ilen := len(icmpdata)
			p2 := make([]byte, 40+ilen)
			copy(p2[0:4], p[0:4])
			binary.BigEndian.PutUint16(p2[4:6], uint16(ilen))
			p2[6] = 58
			p2[7] = 255
			copy(p2[8:24], src2)
			copy(p2[24:40], dst2)
			copy(p2[40:], icmpdata)

			return rnode.SendPlaintext(route, PingMulticodec, p2)
		}

		if p[40] == byte(ipv6.ICMPTypeTimeExceeded) {
			// rnode.Log().Printf("fc00: icmp-time-exceeded from %s @ %s", peerid, route)

			src := peerid.PublicKey().Addr()
			if 0 != bytes.Compare(p[8:24], src) {
				rnode.Log().Printf("fc00: recv: src address mismatch")
				return nil
			}

			dst := rnode.PeerID().PublicKey().Addr()
			if 0 != bytes.Compare(p[24:40], dst) {
				rnode.Log().Printf("fc00: recv: dst address mismatch")
				return nil
			}

			return consumeIncomingPacket(rnode, dev, p, peerid, route)
		}

		// rnode.Log().Printf("fc00: dropping ping packet from %s @ %s %#v", peerid, route, p)
		return nil
	})

	for {
		var buf [1420]byte // rovy.PreliminaryMTU + tunhdrOffset
		n, err := dev.Read(buf[0:], tunhdrOffset)
		if err != nil {
			rnode.Log().Printf("tun: read: %s", err)
			continue
		}

		// rnode.Log().Printf("tun: got %#v", buf)

		// TODO: more checks
		if n < ipv6.HeaderLen+tunhdrOffset {
			rnode.Log().Printf("tun: packet too short (len=%d)", n)
			continue
		}

		ethertype := buf[2:4]
		if 0 != bytes.Compare([]byte{0x86, 0xdd}, ethertype) {
			rnode.Log().Printf("tun: not an ipv6 packet")
			continue
		}

		gotlen := int(binary.BigEndian.Uint16(buf[tunhdrOffset+4 : tunhdrOffset+6]))
		if n != gotlen+ipv6.HeaderLen {
			rnode.Log().Printf("fc00: recv: length mismatch, expected %d, got %d (%d + %d)", n, gotlen+ipv6.HeaderLen, gotlen, ipv6.HeaderLen)
			continue
		}

		nexthdr := buf[tunhdrOffset+6]
		hops := int(buf[tunhdrOffset+7])
		src := net.IP(buf[tunhdrOffset+8 : tunhdrOffset+24])
		dst := net.IP(buf[tunhdrOffset+24 : tunhdrOffset+40])

		// no link-local or multicast stuff yet
		if dst.IsMulticast() || !src.Equal(rnode.PeerID().PublicKey().Addr()) {
			// rnode.Log().Printf("tun: dropping multicast packet %s -> %s", src, dst)
			continue
		}

		peerid, present := addrbook[dst.String()]
		if !present {
			rnode.Log().Printf("tun: no PeerID for %s", dst)
			continue
		}

		route, err := rnode.Routing().GetRoute(peerid)
		if err != nil {
			rnode.Log().Printf("tun: no route for %s: %s", peerid, err)
			continue
		}

		pkt := buf[tunhdrOffset : tunhdrOffset+n]

		// end-to-end transmission
		rlen := route.Len()
		if hops >= rlen {
			err = rnode.SendUpper(peerid, Fc00Multicodec, pkt, route)
			if err != nil {
				rnode.Log().Printf("tun: failed to send: %s", err)
			}
			continue
		}

		// icmp
		if nexthdr == byte(58) {
			err = rnode.SendPlaintext(route[0:hops], PingMulticodec, pkt)
			if err != nil {
				rnode.Log().Printf("tun: failed to send: %s", err)
			}
			continue
		}

		rnode.Log().Printf("tun: fc00 packet %s -> %s dropped (ttl=%d, nexthdr=%d)", src, dst, hops, nexthdr)
	}
	return nil
}

func main() {
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGQUIT)
		buf := make([]byte, 1<<20)
		for {
			<-sigs
			stacklen := runtime.Stack(buf, true)
			log.Printf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end\n", buf[:stacklen])
		}
	}()

	if err := run(); err != nil {
		log.Fatalf(err.Error())
	}
}
