package rovyfc00

import (
	"fmt"
	"log"
	"net"
	"runtime"

	netlink "github.com/vishvananda/netlink"
	netns "github.com/vishvananda/netns"
	tun "golang.zx2c4.com/wireguard/tun"
)

func NetlinkTun(ifname string, ip6 net.IP, mtu int, logger *log.Logger) (Device, error) {
	fd, err := bindTun(ifname)
	if err != nil {
		return nil, err
	}

	dev, _, err := tun.CreateUnmonitoredTUNFromFD(fd)
	if err != nil {
		return nil, fmt.Errorf("CreateUnmonitoredTUNFromFD: %s", err)
	}

	link, err := netlink.LinkByName(ifname)
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("LinkByName: %s", err)
	}

	err = netlink.LinkSetMTU(link, mtu)
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("LinkSetMTU: %s", err)
	}

	nladdr, err := netlink.ParseAddr(ip6.String() + "/128")
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("ParseAddr: %s", err)
	}

	err = netlink.AddrAdd(link, nladdr)
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("AddrAdd: %s", err)
	}

	err = netlink.LinkSetUp(link)
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("LinkSetUp: %s", err)
	}

	_, ip6cidr, err := net.ParseCIDR("fc00::/8")
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("ParseCIDR: %s", err)
	}

	err = netlink.RouteAdd(&netlink.Route{LinkIndex: link.Attrs().Index, Dst: ip6cidr})
	if err != nil {
		logger.Printf("failed to add route %s => %s, skipping", ip6cidr, ifname)
	} else {
		logger.Printf("added route %s => %s", ip6cidr, ifname)
	}

	return Device(dev), nil
}

func NetlinkTunWithNamespace(ifname string, ip6 net.IP, mtu int, logger *log.Logger) (Device, error) {
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

	dev, err := NetlinkTun(ifname, ip6, mtu, logger)
	netns.Set(origns)
	return dev, err
}
