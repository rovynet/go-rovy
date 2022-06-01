package rovygvtun

import (
	"log"
	"net"
	"net/netip"

	wgnetip "golang.zx2c4.com/go118/netip"
	wgtun "golang.zx2c4.com/wireguard/tun"
	wgnet "golang.zx2c4.com/wireguard/tun/netstack"
	gvnet "gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
)

func NewGvisorTUN(ip6a netip.Addr, mtu int, logger *log.Logger) (wgtun.Device, GvisorNet, error) {
	addrs := []wgnetip.Addr{wgnetip.AddrFrom16(ip6a.As16())}
	tdev, tnet, err := wgnet.CreateNetTUN(addrs, addrs, mtu)
	if err != nil {
		return nil, nil, err
	}

	gvnet := &gvisorNet{GvNet: tnet, Logger: logger}
	return tdev, gvnet, nil
}

type GvisorNet interface {
	ListenTCPAddrPort(netip.AddrPort) (*gvnet.TCPListener, error)
	ListenTCP(*net.TCPAddr) (*gvnet.TCPListener, error)
	ListenUDPAddrPort(netip.AddrPort) (*gvnet.UDPConn, error)
	ListenUDP(*net.UDPAddr) (*gvnet.UDPConn, error)
}

type gvisorNet struct {
	GvNet  *wgnet.Net
	Logger *log.Logger
}

func (g *gvisorNet) ListenTCPAddrPort(laddr netip.AddrPort) (*gvnet.TCPListener, error) {
	return g.GvNet.ListenTCPAddrPort(wgnetip.MustParseAddrPort(laddr.String()))
}

func (g *gvisorNet) ListenTCP(laddr *net.TCPAddr) (*gvnet.TCPListener, error) {
	return g.GvNet.ListenTCP(laddr)
}

func (g *gvisorNet) ListenUDPAddrPort(laddr netip.AddrPort) (*gvnet.UDPConn, error) {
	return g.GvNet.ListenUDPAddrPort(wgnetip.MustParseAddrPort(laddr.String()))
}

func (g *gvisorNet) ListenUDP(laddr *net.UDPAddr) (*gvnet.UDPConn, error) {
	return g.GvNet.ListenUDP(laddr)
}
