package fcnet

import (
	"context"
	"fmt"
	"log"
	"net/netip"

	dbus "github.com/godbus/dbus/v5"
	tun "golang.zx2c4.com/wireguard/tun"
)

//
// How to create TUN using nmcli:
//
// Minimal:
// nmcli conn add save no type tun ifname rovy0 con-name rovy0 \
//   ipv4.method disabled ipv6.method disabled
//
// Full:
// nmcli conn add save no type tun ifname rovy0 con-name rovy0 \
//   mtu '1280' ipv4.method 'disabled' \
//   ipv6.method 'manual' ipv6.addresses 'fce2:2cda:998a:5dfc:ccb8:dd48:e541:76cd' \
//   ipv6.routes 'fc00::/8' ipv6.dns 'fc00::1' ipv6.dns-search '~rovy'
//

type NMTUN struct {
	bus    *dbus.Conn
	conn   *dbus.ObjectPath
	dev    tun.Device
	logger *log.Logger
}

func NewNMTUN(logger *log.Logger) *NMTUN {
	nm := &NMTUN{logger: logger}
	return nm
}

func (nm *NMTUN) Start(ifname string, ip netip.Addr, mtu int) error {
	bus, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("dbus: %w", err)
	}
	nm.bus = bus

	devPresent, err := checkTunExists(ifname)
	if err != nil {
		return fmt.Errorf("checkTunExists: %s", err)
	}

	var tunfd int
	if devPresent {
		// if rovy0 is present, we first want to check if it's used by someone else.
		// TODO: consider it could be a multi-queue tun simultaneously used by others
		tunfd, err = bindTun(ifname)
		if err != nil {
			return err
		}
	}

	settingspath, err := nm.updateOrCreateConn(ifname, ip, mtu)
	if err != nil {
		return err
	}

	devpath, err := nm.getDevice(ifname)
	if err != nil {
		return err
	}

	if err := nm.activateConn(settingspath, devpath); err != nil {
		return err
	}

	if tunfd == 0 {
		tunfd, err = bindTun(ifname)
		if err != nil {
			return err
		}
	}
	nm.dev, _, err = tun.CreateUnmonitoredTUNFromFD(tunfd)
	return err
}

func (nm *NMTUN) Device() tun.Device {
	return nm.dev
}

func (nm *NMTUN) Close() error {
	// close nm.tun
	// delete nm.conn
	return nil
}

func (nm *NMTUN) tryBindTUN(ifname string) (int, error) {
	return 0, nil
}

func (nm *NMTUN) bindTUN(ifname string) (int, error) {
	return 0, nil
}

func (nm *NMTUN) updateOrCreateConn(ifname string, ip netip.Addr, mtu int) (dbus.ObjectPath, error) {
	newCfg := nm.prepareConn(ifname, ip, mtu)

	connPath, connCfg, err := nm.findConn(ifname)
	if err != nil {
		return connPath, nil
	}

	if connCfg != nil {
		nm.logger.Printf("reusing NetworkManager connection %s...", ifname)
		connCfg["802-3-ethernet"] = newCfg["802-3-ethernet"]
		connCfg["tun"] = newCfg["tun"]
		connCfg["ipv6"] = newCfg["ipv6"]
		connCfg["ipv4"] = newCfg["ipv4"]
		// delete(connCfg, "addresses") // readonly, deprecated by NM
		// delete(connCfg, "routes")    // readonly, deprecated by NM
		err = nm.updateConn(connPath, connCfg)
	} else {
		nm.logger.Printf("creating NetworkManager connection %s...", ifname)
		connPath, err = nm.createConn(newCfg)
	}

	return connPath, err
}

func (nm *NMTUN) findConn(ifname string) (dbus.ObjectPath, map[string]map[string]dbus.Variant, error) {
	var connPath dbus.ObjectPath
	var connPaths []dbus.ObjectPath

	settingsObj := nm.bus.Object(
		"org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager/Settings")
	m := "org.freedesktop.NetworkManager.Settings.ListConnections"
	err := settingsObj.CallWithContext(context.TODO(), m, 0).Store(&connPaths)
	if err != nil {
		return connPath, nil, fmt.Errorf("ListConnections: %s", err)
	}

	var conn map[string]map[string]dbus.Variant
	for _, path := range connPaths {
		var cfgraw dbus.Variant

		connObj := nm.bus.Object("org.freedesktop.NetworkManager", path)

		m := "org.freedesktop.NetworkManager.Settings.Connection.GetSettings"
		err := connObj.CallWithContext(context.TODO(), m, 0).Store(&cfgraw)
		if err != nil {
			return connPath, nil, fmt.Errorf("GetSettings: %s", err)
		}

		cfg := cfgraw.Value().(map[string]map[string]dbus.Variant)
		if cfgconn, ok := cfg["connection"]; ok {
			if cfgid, ok := cfgconn["interface-name"]; ok {
				if cfgid.Value().(string) == ifname {
					connPath = path
					conn = cfg
					continue
				}
			}
		}
	}
	return connPath, conn, nil
}

func (nm *NMTUN) createConn(cfg map[string]map[string]dbus.Variant) (dbus.ObjectPath, error) {
	var path dbus.ObjectPath

	settingsObj := nm.bus.Object(
		"org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager/Settings")

	m := "org.freedesktop.NetworkManager.Settings.AddConnectionUnsaved"
	err := settingsObj.CallWithContext(context.TODO(), m, 0, cfg).Store(&path)
	if err != nil {
		return path, fmt.Errorf("AddConnectionUnsaved: %s", err)
	}

	return path, nil
}

func (nm *NMTUN) updateConn(path dbus.ObjectPath, cfg map[string]map[string]dbus.Variant) error {
	connObj := nm.bus.Object(
		"org.freedesktop.NetworkManager", path)
	m := "org.freedesktop.NetworkManager.Settings.Connection.UpdateUnsaved"
	call := connObj.CallWithContext(context.TODO(), m, 0, cfg)
	if call.Err != nil {
		return call.Err
	}

	return nil
}

func (nm *NMTUN) getDevice(ifname string) (dbus.ObjectPath, error) {
	var devpath dbus.ObjectPath

	nmObj := nm.bus.Object(
		"org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager")

	m := "org.freedesktop.NetworkManager.GetDeviceByIpIface"
	err := nmObj.CallWithContext(context.TODO(), m, 0, ifname).Store(&devpath)
	if err != nil {
		return devpath, fmt.Errorf("GetDeviceByIpIface: %s", err)
	}

	return devpath, nil
}

func (nm *NMTUN) activateConn(conn, device dbus.ObjectPath) error {
	var activeconnpath dbus.ObjectPath

	nmObj := nm.bus.Object(
		"org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager")
	nothin := dbus.ObjectPath("/")

	m := "org.freedesktop.NetworkManager.ActivateConnection"
	err := nmObj.CallWithContext(context.TODO(), m, 0, conn, device, nothin).Store(&activeconnpath)
	if err != nil {
		return fmt.Errorf("ActivateConnection: %s", err)
	}

	return nil
}

func (nm *NMTUN) prepareConn(ifname string, ip netip.Addr, mtu int) map[string]map[string]dbus.Variant {
	return map[string]map[string]dbus.Variant{
		"connection": {
			"id":             dbus.MakeVariant(ifname),
			"interface-name": dbus.MakeVariant(ifname),
			"type":           dbus.MakeVariant("tun"),
		},
		"802-3-ethernet": {
			"mtu": dbus.MakeVariant(uint32(mtu)),
		},
		"tun": {
			"pi": dbus.MakeVariant(false),
		},
		"ipv6": {
			"method": dbus.MakeVariant("manual"),
			"address-data": dbus.MakeVariant([]map[string]interface{}{
				{"address": ip.String(), "prefix": uint32(128)},
			}),
			"route-data": dbus.MakeVariant([]map[string]interface{}{
				{"dest": "fc00::", "prefix": uint32(8)},
			}),
			"dns":        dbus.MakeVariant([][]byte{netip.MustParseAddr("fc00::1").AsSlice()}),
			"dns-search": dbus.MakeVariant([]string{"~rovy."}),
		},
		"ipv4": {
			"method": dbus.MakeVariant("disabled"),
		},
	}
}
