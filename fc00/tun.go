package rovyfc00

import (
	"fmt"
	"net"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"

	tun "golang.zx2c4.com/wireguard/tun"
)

type Device tun.Device

func PreconfiguredTUN(ifname string) (Device, error) {
	fd, err := bindTun(ifname)
	if err != nil {
		return nil, err
	}
	return FileTUN(fd)
}

func FileTUN(fd int) (Device, error) {
	dev, _, err := tun.CreateUnmonitoredTUNFromFD(fd)
	return Device(dev), err
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
