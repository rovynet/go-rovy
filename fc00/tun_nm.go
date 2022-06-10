package rovyfc00

import (
	"context"
	"fmt"
	"log"
	"net/netip"

	"golang.org/x/sys/unix"

	dbus "github.com/godbus/dbus/v5"
	tun "golang.zx2c4.com/wireguard/tun"
)

// How to create TUN using nmcli:
//
// Minimal: nmcli conn add type tun ifname rovy0 con-name rovy0 ipv4.method disabled ipv6.method disabled
//
// Full: nmcli conn add save no type tun ifname rovy0 con-name rovy0 mtu '1280' ipv4.method 'disabled' ipv6.method 'manual' ipv6.addresses 'fce2:2cda:998a:5dfc:ccb8:dd48:e541:76cd' ipv6.routes 'fc00::/8' ipv6.dns-search '~rovy' ipv6.dns 'fc00::1'

// How to query DBus properties:
//
// devconns, err := sysbus.Object(nmdest, settingspath).GetProperty(
// 	nmdest+".Device.AvailableConnections")
// if err != nil {
// 	return nil, fmt.Errorf("AvailableConnections: %s", err)
// }
// logger.Printf("devconns=%+v", devconns.Value().([]dbus.ObjectPath))

const cloneDevicePath = "/dev/net/tun"
const ifReqSize = unix.IFNAMSIZ + 64
const nmdest = "org.freedesktop.NetworkManager"

func NetworkManagerTun(ifname string, ip6 netip.Addr, mtu int, logger *log.Logger) (tun.Device, error) {
	devPresent, err := checkTunExists(ifname)
	if err != nil {
		return nil, fmt.Errorf("checkTunExists: %s", err)
	}

	sysbus, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("dbus: %w", err)
	}
	nmpath := dbus.ObjectPath("/org/freedesktop/NetworkManager")
	nullpath := dbus.ObjectPath("/")

	var tunfd int
	if devPresent {
		// if rovy0 is present, we first want to check if it's used by someone else.
		// TODO: consider it could be a multi-queue tun simultaneously used by others
		logger.Printf("tun interface %s exists, trying to reuse it...", ifname)
		tunfd, err = bindTun(ifname)
		if err != nil {
			return nil, err
		}
	} else {
		logger.Printf("tun interface %s doesn't exist, have NetworkManager create it...", ifname)
	}

	// // TODO: first check if there's an active connection we must reuse
	// connpath, err = getNMConn(sysbus, ifname) // NetworkManager/Device/%d
	// connPresent := true
	// if err != nil {
	// 	connPresent = false
	// }

	settingspath, err := createNMConn(sysbus, ifname, ip6, mtu)
	if err != nil {
		return nil, fmt.Errorf("createNMConn: %s", err)
	}
	if tunfd == 0 {
		tunfd, err = bindTun(ifname)
		if err != nil {
			return nil, err
		}
	}

	// logger.Printf("tunfd=%+v settingspath=%+v", tunfd, settingspath)

	devpath, err := getNMDevice(sysbus, ifname) // NetworkManager/Device/%d
	if err != nil {
		return nil, err
	}
	// logger.Printf("devpath=%+v", devpath)

	var activeconnpath dbus.ObjectPath
	err = sysbus.Object(nmdest, nmpath).CallWithContext(context.TODO(),
		nmdest+".ActivateConnection", 0, settingspath, devpath, nullpath,
	).Store(&activeconnpath)
	if err != nil {
		return nil, fmt.Errorf("ActivateConnection: %s", err)
	}
	// logger.Printf("activeconnpath=%+v", activeconnpath)

	dev, _, err := tun.CreateUnmonitoredTUNFromFD(tunfd)
	return dev, err
}

// returns path to /org/freedesktop/NetworkManager/Settings/%d
func createNMConn(bus *dbus.Conn, ifname string, ip6 netip.Addr, mtu int) (dbus.ObjectPath, error) {
	nm := bus.Object(
		nmdest,
		dbus.ObjectPath("/org/freedesktop/NetworkManager/Settings"),
	)

	settings := getNMConnSettings(ifname, ip6, mtu)
	var settingspath dbus.ObjectPath
	err := nm.CallWithContext(context.TODO(),
		nmdest+".Settings.AddConnectionUnsaved", 0, settings,
	).Store(&settingspath)
	if err != nil {
		return settingspath, fmt.Errorf("AddConnectionUnsaved: %s", err)
	}
	return settingspath, nil
}

func getNMDevice(bus *dbus.Conn, ifname string) (dbus.ObjectPath, error) {
	nm := bus.Object(
		nmdest,
		dbus.ObjectPath("/org/freedesktop/NetworkManager"),
	)

	var devpath dbus.ObjectPath
	err := nm.CallWithContext(context.TODO(),
		nmdest+".GetDeviceByIpIface", 0, ifname,
	).Store(&devpath)
	if err != nil {
		return devpath, fmt.Errorf("GetDeviceByIpIface: %s", err)
	}

	return devpath, nil
}

type nmConnectionSettings map[string]map[string]dbus.Variant

func getNMConnSettings(ifname string, ip6 netip.Addr, mtu int) nmConnectionSettings {
	dns6 := netip.MustParseAddr("fc00::1").As16()
	return nmConnectionSettings{
		"connection": map[string]dbus.Variant{
			"id":             dbus.MakeVariant(ifname),
			"interface-name": dbus.MakeVariant(ifname),
			"type":           dbus.MakeVariant("tun"),
		},
		"802-3-ethernet": map[string]dbus.Variant{
			"mtu": dbus.MakeVariant(uint32(mtu)),
		},
		"tun": map[string]dbus.Variant{
			"pi": dbus.MakeVariant(false),
		},
		"ipv6": map[string]dbus.Variant{
			"method": dbus.MakeVariant("manual"),
			"address-data": dbus.MakeVariant([]map[string]interface{}{
				{"address": ip6.String(), "prefix": uint32(128)},
			}),
			"route-data": dbus.MakeVariant([]map[string]interface{}{
				{"dest": "fc00::", "prefix": uint32(8)},
			}),
			"dns":        dbus.MakeVariant([][]byte{dns6[:]}),
			"dns-search": dbus.MakeVariant([]string{"~rovy."}),
		},
		"ipv4": map[string]dbus.Variant{
			"method": dbus.MakeVariant("disabled"),
		},
	}
}
