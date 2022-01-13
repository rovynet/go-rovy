package node

import (
	"errors"
	"fmt"

	rovy "go.rovy.net"
	forwarder "go.rovy.net/forwarder"
	session "go.rovy.net/session"
)

var ErrDontRelease = errors.New("pls dont release this buffer yet, we handed it off")

// hello send

func (node *Node) helloSendRoutine() {
	for {
		pkt := node.helloSendQ.Get()

		var fn func(rovy.Packet) error
		if pkt.LowerDst.Empty() {
			fn = node.doUpperHelloSend
		} else {
			fn = node.doLowerHelloSend
		}

		err := fn(pkt)
		if err == ErrDontRelease {
			continue
		}
		if err != nil {
			node.Log().Printf("helloSendRoutine: %s", err)
		}
		node.ReleasePacket(pkt)
	}
}

func (node *Node) doLowerHelloSend(pkt rovy.Packet) error {
	if pkt.TptDst.Empty() {
		return fmt.Errorf("lower packet without TptDst")
	}
	if pkt.LowerDst.Empty() {
		return fmt.Errorf("lower packet without LowerDst")
	}

	hellopkt := session.NewHelloPacket(pkt, rovy.LowerOffset, rovy.LowerPadding)
	hellopkt, err := node.SessionManager().CreateHello(hellopkt, pkt.LowerDst, pkt.TptDst)
	if err != nil {
		return err
	}

	if err := node.sendTransport(hellopkt.Packet); err != nil {
		return err
	}

	return ErrDontRelease
}

func (node *Node) doUpperHelloSend(pkt rovy.Packet) error {
	if pkt.UpperDst.Empty() {
		return fmt.Errorf("upper packet without UpperDst")
	}

	hellopkt := session.NewHelloPacket(pkt, rovy.UpperOffset, rovy.UpperPadding)
	hellopkt, err := node.SessionManager().CreateHello(hellopkt, pkt.UpperDst, rovy.UDPMultiaddr{})
	if err != nil {
		return err
	}

	// TODO: route lookup should move to where the packet is enqueued
	route, err := node.Routing().GetRoute(pkt.UpperDst)
	if err != nil {
		return err
	}
	upkt := rovy.NewUpperPacket(hellopkt.Packet)
	upkt.SetRoute(route)

	if err = node.Forwarder().SendPacket(upkt); err != nil {
		return fmt.Errorf("forwarder: %s", err)
	}

	return ErrDontRelease
}

// lower send

func (node *Node) lowerSendRoutine() {
	for {
		pkt := node.lowerSendQ.Get()

		err := node.doLowerSend(pkt)
		if err == ErrDontRelease {
			continue
		}
		if err != nil {
			node.Log().Printf("lowerSendRoutine: %s", err)
		}
		node.ReleasePacket(pkt)
	}
}

func (node *Node) doLowerSend(pkt rovy.Packet) error {
	if pkt.LowerDst.Empty() {
		return fmt.Errorf("lowerSendRoutine: dropping packet without LowerDst")
	}

	datapkt := session.NewDataPacket(pkt, rovy.LowerOffset, rovy.LowerPadding)

	raddr, err := node.SessionManager().CreateData(datapkt, datapkt.LowerDst)
	if err != nil {
		return err
	}

	datapkt.TptDst = raddr
	if err := node.sendTransport(datapkt.Packet); err != nil {
		return err
	}

	return ErrDontRelease
}

// upper send

func (node *Node) upperSendRoutine() {
	for {
		pkt := node.upperSendQ.Get()

		err := node.doUpperSend(pkt)
		if err == ErrDontRelease {
			continue
		}
		if err != nil {
			node.Log().Printf("upperSendRoutine: %s", err)
		}
		node.ReleasePacket(pkt)
	}
}

func (node *Node) doUpperSend(pkt rovy.Packet) error {
	if pkt.UpperDst.Empty() {
		return fmt.Errorf("upperSendRoutine: packet without UpperDst")
	}

	upkt := rovy.NewUpperPacket(pkt)

	if upkt.RouteLen() == forwarder.HopLength {
		lpkt := rovy.NewLowerPacket(upkt.Packet)
		lpkt.SetCodec(DirectUpperCodec)
		lpkt.LowerDst = upkt.UpperDst
		node.lowerSendQ.PutWithBackpressure(lpkt.Packet)
		return nil
	}

	datapkt := session.NewDataPacket(upkt.Packet, rovy.UpperOffset, rovy.UpperPadding)
	_, err := node.SessionManager().CreateData(datapkt, datapkt.UpperDst)
	if err != nil {
		return err
	}

	if err = node.Forwarder().SendPacket(upkt); err != nil {
		return fmt.Errorf("forwarder: %s", err)
	}

	return ErrDontRelease
}
