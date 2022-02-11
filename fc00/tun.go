package fc00

import (
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"

	netlink "github.com/vishvananda/netlink"
	netns "github.com/vishvananda/netns"
	tun "golang.zx2c4.com/wireguard/tun"
)

func DefaultTun(ifname string, ip6 net.IP, mtu int, logger *log.Logger) (tun.Device, error) {
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

func checkTunExists(ifname string) (bool, error) {
	_, err := net.InterfaceByName(ifname)
	if err != nil {
		if err.(*net.OpError).Err.Error() == "no such network interface" {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// taken from wireguard-go/tun/tun_linux.go (MIT-licensed)
func bindTun(ifname string) (int, error) {
	nfd, err := unix.Open(cloneDevicePath, os.O_RDWR, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("bindTun: %s does not exist", cloneDevicePath)
		}
		return 0, err
	}

	var ifr [ifReqSize]byte
	var flags uint16 = unix.IFF_TUN | unix.IFF_NO_PI
	nameBytes := []byte(ifname)
	if len(nameBytes) >= unix.IFNAMSIZ {
		unix.Close(nfd)
		return 0, fmt.Errorf("bindTun: ifname too long: %s - %w", ifname, unix.ENAMETOOLONG)
	}
	copy(ifr[:], nameBytes)
	*(*uint16)(unsafe.Pointer(&ifr[unix.IFNAMSIZ])) = flags

	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(nfd),
		uintptr(unix.TUNSETIFF),
		uintptr(unsafe.Pointer(&ifr[0])),
	)
	if errno != 0 {
		unix.Close(nfd)
		return 0, errno
	}

	err = unix.SetNonblock(nfd, true)
	if err != nil {
		unix.Close(nfd)
		return 0, err
	}

	return nfd, nil
}
