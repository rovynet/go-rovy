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

		for _, a := range pkt.Addrs {
			a.IP = a.IP.WithZone(raddr.Addr().Zone())
			a.PeerID = pkt.PeerID
			ll.Log.Printf("discovery: found %s", a)
		}
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
			// get interface names and respective link-local addresses
			ifaces, err := ll.linklocalCapableInterfaces()
			if err != nil {
				ll.Log.Printf("discovery: linklocal: %s", err)
				continue
			}

			// get listening port
			var ourport uint16
			// ourport = uint16(12345)
			status, err := ll.API.Peer().Status()
			if err != nil {
				ll.Log.Printf("discovery: peer/status: %s", err)
				continue
			}
			for _, listener := range status.Listeners {
				if listener.ListenAddr.IP.Is6() {
					ourport = listener.ListenAddr.Port
					break
				}
			}

			// announce on each capable interface
			for ifname, ouraddr := range ifaces {
				ni, _ := ll.API.Info()
				pkt := LinkLocalPacket{
					PeerID: ni.PeerID,
					Addrs:  []rovy.Multiaddr{rovy.Multiaddr{IP: ouraddr, Port: ourport}},
				}
				buf, err := cbor.Marshal(pkt)
				if err != nil {
					ll.Log.Printf("discovery: cbor: %s", err)
					break
				}

				addr := netip.AddrPortFrom(netip.IPv6LinkLocalAllNodes().WithZone(ifname), LinkLocalPort)
				if _, err = conn.WriteToUDPAddrPort(buf, addr); err != nil {
					ll.Log.Printf("discovery: %s", err)
				}
			}
		}
	}
}

func (ll *LinkLocal) linklocalCapableInterfaces() (map[string]netip.Addr, error) {
	out := make(map[string]netip.Addr)
	fcpref := netip.MustParsePrefix("fc00::/8")

	ifaces, err := net.Interfaces()
	if err != nil {
		return out, fmt.Errorf("discovery: error getting network interfaces: %s", err)
	}

	for _, iface := range ifaces {
		ifaddrs, err := iface.Addrs()
		if err != nil {
			return out, fmt.Errorf("discovery: error getting interface addresses: %s", err)
		}

		capable := false
		isrovy := false
		var addr netip.Addr
		for _, ifaddr := range ifaddrs {
			pref, err := netip.ParsePrefix(ifaddr.String())
			if err != nil {
				return out, fmt.Errorf("discovery: error parsing address: %s", err)
			}
			a := pref.Addr()

			if fcpref.Contains(a) {
				isrovy = true
			}

			if a.Is6() && a.IsLinkLocalUnicast() {
				capable = true
				addr = a
			}
		}
		if capable && !isrovy {
			out[iface.Name] = addr
		}
	}

	return out, nil
}
