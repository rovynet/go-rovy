package node

import (
	rapi "go.rovy.net/api"
	rdisco "go.rovy.net/node/discovery"
)

type DiscoveryAPI Node

func (c *DiscoveryAPI) Status() (rapi.DiscoveryStatus, error) {
	return rapi.DiscoveryStatus{}, nil
}

func (c *DiscoveryAPI) StartLinkLocal(opts rapi.DiscoveryLinkLocal) error {
	sm := (*Node)(c).Services()

	ll := &rdisco.LinkLocal{
		API:      c.NodeAPI(),
		Interval: opts.Interval,
		Log:      (*Node)(c).Log(),
	}
	err := sm.Add(rdisco.ServiceTagLinkLocal, ll)
	if err != nil {
		return err
	}

	return sm.Start(rdisco.ServiceTagLinkLocal)
}

func (c *DiscoveryAPI) StopLinkLocal() error {
	return (*Node)(c).Services().Stop(rdisco.ServiceTagLinkLocal)
}

func (c *DiscoveryAPI) NodeAPI() rapi.NodeAPI {
	return (*Node)(c)
}
