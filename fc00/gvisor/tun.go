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

func NewGvisorTUN(ip6a netip.Addr, mtu int, logger *log.Logger) (wgtun.Device, *GvisorNet, error) {
	addrs := []wgnetip.Addr{wgnetip.AddrFrom16(ip6a.As16())}
	tdev, tnet, err := wgnet.CreateNetTUN(addrs, addrs, mtu)
	if err != nil {
		return nil, nil, err
	}

	gvnet := &GvisorNet{GvNet: tnet, Logger: logger}
	return tdev, gvnet, nil
}

type GvisorNet struct {
	GvNet  *wgnet.Net
	Logger *log.Logger
}

func (g *GvisorNet) ListenTCPAddrPort(laddr wgnetip.AddrPort) (*gvnet.TCPListener, error) {
	return g.GvNet.ListenTCPAddrPort(wgnetip.AddrPort(laddr))
}

func (g *GvisorNet) ListenTCP(laddr *net.TCPAddr) (*gvnet.TCPListener, error) {
	return g.GvNet.ListenTCP(laddr)
}

func (g *GvisorNet) ListenUDPAddrPort(laddr wgnetip.AddrPort) (*gvnet.UDPConn, error) {
	return g.GvNet.ListenUDPAddrPort(wgnetip.AddrPort(laddr))
}

func (g *GvisorNet) ListenUDP(laddr *net.UDPAddr) (*gvnet.UDPConn, error) {
	return g.GvNet.ListenUDP(laddr)
}
