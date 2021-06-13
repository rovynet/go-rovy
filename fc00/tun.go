package fc00

import (
	"log"
	"net"
	"runtime"

	netlink "github.com/vishvananda/netlink"
	netns "github.com/vishvananda/netns"
	tun "golang.zx2c4.com/wireguard/tun"
)

func DefaultTun(ifname string, ip6 net.IP, mtu int, logger *log.Logger) (tun.Device, error) {
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
		logger.Printf("failed to add route %s => %s, skipping", ip6cidr, ifname)
	} else {
		logger.Printf("added route %s => %s", ip6cidr, ifname)
	}

	return dev, nil
}

func DefaultTunWithNamespace(ifname string, ip6 net.IP, mtu int, logger *log.Logger) (tun.Device, error) {
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

	dev, err := DefaultTun(ifname, ip6, mtu, logger)
	netns.Set(origns)
	return dev, err
}
