package rnodecfg

import (
	"fmt"
	"log"
	"time"

	rovy "go.rovy.net"
	rapi "go.rovy.net/api"
	rconfig "go.rovy.net/api/config"
	fcnet "go.rovy.net/fcnet"
	rnode "go.rovy.net/node"
)

type NodeConfig struct {
	API    rapi.NodeAPI
	Logger *log.Logger
}

func (nc *NodeConfig) ConfigureAll(cfg *rconfig.Config, node *rnode.Node) error {
	if err := nc.ConfigurePeering(cfg); err != nil {
		return fmt.Errorf("error configuring peering: %s", err)
	}

	if err := nc.ConfigureFcnet(cfg, node); err != nil {
		return fmt.Errorf("error configuring fcnet: %s", err)
	}

	if err := nc.ConfigureDiscoveryLinkLocal(cfg); err != nil {
		return fmt.Errorf("error configuring discovery: %s", err)
	}

	return nil
}

// TODO: do the actual configuration using api/client module
func (nc *NodeConfig) ConfigurePeering(cfg *rconfig.Config) error {
	for _, addr := range cfg.Peer.Listen {
		_, err := nc.API.Peer().Listen(addr)
		if err != nil {
			return err
		}
	}
	for _, addr := range cfg.Peer.Connect {
		_, err := nc.API.Peer().Connect(addr)
		if err != nil {
			return err
		}
	}
	return nil
}

// TODO: make use of actual config
// TODO: close our FD?
func (nc *NodeConfig) ConfigureFcnet(cfg *rconfig.Config, node *rnode.Node) error {
	if !cfg.Fcnet.Enabled {
		return nil
	}

	nm := fcnet.NewNMTUN(nc.Logger)
	if err := nm.Start(cfg.Fcnet.Ifname, node.IPAddr(), rovy.UpperMTU); err != nil {
		return fmt.Errorf("networkmanager: %s", err)
	}

	tunfd := nm.Device().File()

	if err := nc.API.Fcnet().Start(tunfd); err != nil {
		return fmt.Errorf("api: %s", err)
	}

	nc.Logger.Printf("started fcnet endpoint %s using NetworkManager", node.IPAddr())

	return nil
}

func (nc *NodeConfig) ConfigureDiscoveryLinkLocal(cfg *rconfig.Config) error {
	if !cfg.Discovery.LinkLocal.Enabled {
		return nil
	}

	interval, err := time.ParseDuration(cfg.Discovery.LinkLocal.Interval)
	if err != nil {
		return fmt.Errorf("config: ParseDuration interval: %s", err)
	}

	opts := rapi.DiscoveryLinkLocal{
		Interval: interval.Abs(),
	}
	if err := nc.API.Discovery().StartLinkLocal(opts); err != nil {
		return fmt.Errorf("api: %s", err)
	}

	return nil
}
