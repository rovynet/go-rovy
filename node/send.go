package node

import (
	"fmt"

	rovy "go.rovy.net"
	forwarder "go.rovy.net/forwarder"
	session "go.rovy.net/session"
)

const ErrDontRelease = -2

// hello send

func (node *Node) helloSendRoutine() {
	for {
		pkt := node.helloSendQ.Get()

		if pkt.LowerDst.Empty() {
			if err := node.doUpperHelloSend(pkt); err != nil {
				node.Log().Printf("helloSendRoutine: %s", err)
				continue
			}
		} else {
			if err := node.doLowerHelloSend(pkt); err != nil {
				node.Log().Printf("helloSendRoutine: %s", err)
				continue
			}
		}
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

	return node.sendTransport(hellopkt.Packet)
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
	return nil
}

// lower send

func (node *Node) lowerSendRoutine() {
	for {
		pkt := node.lowerSendQ.Get()

		if err := node.doLowerSend(pkt); err != nil {
			node.Log().Printf("lowerSendRoutine: %s", err)
			continue
		}
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
	return node.sendTransport(datapkt.Packet)
}

// upper send

func (node *Node) upperSendRoutine() {
	for {
		pkt := node.upperSendQ.Get()

		if err := node.doUpperSend(pkt); err != nil {
			node.Log().Printf("upperSendRoutine: %s", err)
			continue
		}
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
	return nil
}
