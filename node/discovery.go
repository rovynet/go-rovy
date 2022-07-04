package rovynode

type DiscoveryService interface {
	Start(context.Context, Node)
	Status() any
}

type DiscoveryAPI Node

func (c *DiscoveryAPI) Status() (rovyapi.DiscoveryStatus, error) {
	return DiscoveryStatus{}, nil
}

// TODO: here goes the discovery goroutine shebang
func (c *DiscoveryAPI) StartLinkLocal(opts rovyapi.DiscoveryLinkLocal) error {
	return nil
}

func (c *DiscoveryAPI) StopLinkLocal() error {
	return nil
}

func (c *DiscoveryAPI) NodeAPI() rovyapi.NodeAPI {
	return (*Node)(c)
}
