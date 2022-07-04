package discovery

import (
	"context"

	rovy "go.rovy.net"
	rovyapi "go.rovy.net/api"
)

type Discovery interface {
	Start(context.Context, rovyapi.NodeAPI) error
	Status() any
}

func (ll *LinkLocal) Start(ctx context.Context, api rovyapi.NodeAPI) error {
	var conns []*net.UDPConn
	for _, ifname := range ll.Interfaces {
		conn, err := net.ListenUDP("udp6", `ff02::1%`+ifname)
		if err != nil {
			for _, c := range conns {
				conn.Close()
			}
			return err
		}
		conns = append(conns, conn)
	}

	go announceRoutine(ctx, conns, api)
	go receiveRoutine(ctx, conns, api.Peer())
	return nil
}

// XXX select on recvmsg and context cancellation
func (ll *LinkLocal) announceRoutine(ctx context.Context) {
}

// XXX select on timer interval and context cancellation
func (ll *LinkLocal) receiveRoutine(ctx context.Context) {}
