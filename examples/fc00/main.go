package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"net"
	"os"
	"runtime"

	ipv6 "golang.org/x/net/ipv6"

	multiaddr "github.com/multiformats/go-multiaddr"
	netlink "github.com/vishvananda/netlink"
	netns "github.com/vishvananda/netns"
	tun "golang.zx2c4.com/wireguard/tun"

	rovy "pkt.dev/go-rovy"
	node "pkt.dev/go-rovy/node"
)

const Fc00Multicodec = 0x42004

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
		// dev.Close()
		// return nil, err
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

	nodeA, err := newNode("nodeA", addrA)
	if err != nil {
		return err
	}
	nodeB, err := newNode("nodeB", addrB)
	_ = nodeB
	if err != nil {
		return err
	}

	devA, err := newTun("rovy0", nodeA.PeerID().PublicKey().Addr(), rovy.PreliminaryMTU, nodeA.Log())
	if err != nil {
		return err
	}

	devB, err := newTunNamespace("rovy1", nodeB.PeerID().PublicKey().Addr(), rovy.PreliminaryMTU, nodeB.Log())
	if err != nil {
		return err
	}

	// ordering is important, need to register multicodec before establishing any sessions
	go doTheThing(nodeA, devA)
	go doTheThing(nodeB, devB)

	if err := nodeA.Connect(nodeB.PeerID(), addrB); err != nil {
		nodeA.Log().Printf("failed to connect nodeA to nodeB: %s", err)
		return err
	}

	select {}
	return nil
}

func doTheThing(rnode *node.Node, dev tun.Device) error {
	tunhdrOffset := 4
	ouraddr := rnode.PeerID().PublicKey().Addr()

	rnode.Handle(Fc00Multicodec, func(p []byte, peerid rovy.PeerID) error {
		// rnode.Log().Printf("fc00: handle %#v", p)

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
		if 0 != bytes.Compare(p[24:40], ouraddr) {
			rnode.Log().Printf("fc00: recv: dst address mismatch")
			return nil
		}

		src := net.IP(p[8:24])
		dst := net.IP(p[24:40])
		rnode.Log().Printf("fc00: received packet %s <- %s", dst, src)

		p = append([]byte{0x0, 0x0, 0x86, 0xdd}, p...) // XXX slowness

		_, err := dev.Write(p, tunhdrOffset)
		return err
	})

	for {
		var buf [1420]byte // rovy.PreliminaryMTU + tunhdrOffset
		n, err := dev.Read(buf[0:], tunhdrOffset)
		if err != nil {
			rnode.Log().Printf("tun: read: %s", err)
			continue
		}
		// n += tunhdrOffset

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

		// buf = buf[tunhdrOffset:]
		// n = n - tunhdrOffset

		gotlen := int(binary.BigEndian.Uint16(buf[tunhdrOffset+4 : tunhdrOffset+6]))
		if n != gotlen+ipv6.HeaderLen {
			rnode.Log().Printf("fc00: recv: length mismatch, expected %d, got %d (%d + %d)", n, gotlen+ipv6.HeaderLen, gotlen, ipv6.HeaderLen)
			continue
		}

		// nexthdr := buf[tunhdrOffset+6]
		// hops := buf[tunhdrOffset+7]
		src := net.IP(buf[tunhdrOffset+8 : tunhdrOffset+24])
		dst := net.IP(buf[tunhdrOffset+24 : tunhdrOffset+40])

		if dst.IsMulticast() {
			// handle multicast, e.g. fan out to all peers
			// rnode.Log().Printf("tun: multicast packet %s -> %s (nexthdr=%d hops=%d)", src, dst, nexthdr, hops)
			continue
		}

		if !src.Equal(ouraddr) {
			rnode.Log().Printf("tun: dropping non-fc00::/8 packet %s -> %s", src, dst)
			continue
		}

		peerid, present := addrbook[dst.String()]
		if !present {
			rnode.Log().Printf("tun: no PeerID for %s", dst)
			continue
		}

		if err = rnode.Send(peerid, Fc00Multicodec, buf[tunhdrOffset:tunhdrOffset+n]); err != nil {
			rnode.Log().Printf("tun: failed to send: %s", err)
			continue
		}

		rnode.Log().Printf("tun: fc00 packet %s -> %s sent to /rovy/%s (len=%d)", src, dst, peerid, len(buf[tunhdrOffset:]))
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf(err.Error())
	}
}
