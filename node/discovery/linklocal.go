package rdiscovery

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"time"

	cbor "github.com/fxamacker/cbor/v2"

	rovy "go.rovy.net"
	rapi "go.rovy.net/api"
	rservice "go.rovy.net/node/service"
)

const (
	ServiceTagLinkLocal = "/rovyservice/linklocal"
	LinkLocalPort       = 12344
)

// TODO: needs a multicodec header
type LinkLocalPacket struct {
	PeerID rovy.PeerID
	Addrs  []rovy.Multiaddr
}

type LinkLocal struct {
	API      rapi.NodeAPI
	Interval time.Duration
	Log      *log.Logger
	running  chan int
}

func (ll *LinkLocal) Start() error {
	if ll.Running() {
		return rservice.ErrServiceRunning
	}
	ll.running = make(chan int)

	addr := netip.AddrPortFrom(netip.IPv6Unspecified(), LinkLocalPort)
	conn, err := net.ListenUDP("udp6", net.UDPAddrFromAddrPort(addr))
	if err != nil {
		return fmt.Errorf("discovery: %s", err)
	}

	go ll.receiveRoutine(conn)
	go ll.announceRoutine(conn)

	return nil
}

func (ll *LinkLocal) Stop() error {
	if !ll.Running() {
		return rservice.ErrServiceNotRunning
	}
	close(ll.running)

	return nil
}

func (ll *LinkLocal) Running() bool {
	if ll.running != nil {
		select {
		case <-ll.running:
			return false
		default:
			return true
		}
	}
	return false
}

func (ll *LinkLocal) receiveRoutine(conn *net.UDPConn) {
	defer conn.Close()

	for {
		if !ll.Running() {
			ll.Log.Printf("discovery: shutting down linklocal receiveRoutine")
			return
		}

		b := make([]byte, 1280)
		n, raddr, err := conn.ReadFromUDPAddrPort(b)
		if err == net.ErrClosed {
			return
		}
		if err != nil {
			ll.Log.Printf("discovery: linklocal: error reading: %s", err)
			continue
		}

		if !raddr.Addr().Is6() || !raddr.Addr().IsLinkLocalUnicast() {
			continue
		}

		var pkt LinkLocalPacket
		if err != cbor.Unmarshal(b[:n], &pkt) {
			ll.Log.Printf("discovery: %s", err)
			continue
		}

		maddr := rovy.Multiaddr{IP: raddr.Addr(), Port: raddr.Port(), PeerID: pkt.PeerID}
		ll.Log.Printf("discovery: found %s", maddr)
	}
}

func (ll *LinkLocal) announceRoutine(conn *net.UDPConn) {
	ticker := time.NewTicker(ll.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ll.running:
			ll.Log.Printf("discovery: shutting down linklocal announceRoutine")
			return
		case <-ticker.C:
			ifnames, err := ll.linklocalCapableInterfaces()
			if err != nil {
				ll.Log.Printf("discovery: linklocal: %s", err)
				continue
			}
			for _, ifname := range ifnames {
				addr := netip.AddrPortFrom(netip.IPv6LinkLocalAllNodes().WithZone(ifname), LinkLocalPort)

				ni, _ := ll.API.Info()
				buf, err := cbor.Marshal(LinkLocalPacket{PeerID: ni.PeerID})
				if err != nil {
					ll.Log.Printf("discovery: cbor: %s", err)
					break
				}

				if _, err = conn.WriteToUDPAddrPort(buf, addr); err != nil {
					ll.Log.Printf("discovery: %s", err)
				}
			}
		}
	}
}

func (ll *LinkLocal) linklocalCapableInterfaces() ([]string, error) {
	links := []string{}
	fcpref := netip.MustParsePrefix("fc00::/8")

	ifaces, err := net.Interfaces()
	if err != nil {
		return links, fmt.Errorf("discovery: error getting network interfaces: %s", err)
	}

	for _, iface := range ifaces {
		ifaddrs, err := iface.Addrs()
		if err != nil {
			return links, fmt.Errorf("discovery: error getting interface addresses: %s", err)
		}

		capable := false
		isrovy := false
		for _, ifaddr := range ifaddrs {
			pref, err := netip.ParsePrefix(ifaddr.String())
			if err != nil {
				return links, fmt.Errorf("discovery: error parsing address: %s", err)
			}
			addr := pref.Addr()

			if fcpref.Contains(addr) {
				isrovy = true
			}

			if addr.Is6() && addr.IsLinkLocalUnicast() {
				capable = true
			}
		}
		if capable && !isrovy {
			links = append(links, iface.Name)
		}
	}

	return links, nil
}
