package node

import (
	"fmt"

	rovy "go.rovy.net"
	forwarder "go.rovy.net/node/forwarder"
	session "go.rovy.net/node/session"
)

// hello send

func (node *Node) helloSendRoutine() {
	for {
		pkt := node.helloSendQ.Get()

		if pkt.LowerDst.Empty() {
			if pkt.UpperDst.Empty() {
				node.Log().Printf("helloSendRoutine: upper packet without UpperDst")
				continue
			}
			if err := node.doUpperHelloSend(pkt); err != nil {
				node.Log().Printf("helloSendRoutine: %s", err)
				continue
			}
		} else {
			if pkt.TptDst.Empty() {
				node.Log().Printf("helloSendRoutine: lower packet without TptDst")
				continue
			}
			if pkt.LowerDst.Empty() {
				node.Log().Printf("helloSendRoutine: lower packet without LowerDst")
				continue
			}
			if err := node.doLowerHelloSend(pkt); err != nil {
				node.Log().Printf("helloSendRoutine: %s", err)
				continue
			}
		}
	}
}

func (node *Node) doLowerHelloSend(pkt rovy.Packet) error {
	hellopkt := session.NewHelloPacket(pkt, rovy.LowerOffset, rovy.LowerPadding)
	hellopkt, err := node.SessionManager().CreateHello(hellopkt, pkt.LowerDst, pkt.TptDst)
	if err != nil {
		return err
	}

	node.sendTransport(hellopkt.Packet)

	return nil
}

func (node *Node) doUpperHelloSend(pkt rovy.Packet) error {
	hellopkt := session.NewHelloPacket(pkt, rovy.UpperOffset, rovy.UpperPadding)
	hellopkt, err := node.SessionManager().CreateHello(hellopkt, pkt.UpperDst, rovy.Multiaddr{})
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

		if pkt.LowerDst.Empty() {
			node.Log().Printf("lowerSendRoutine: dropping packet without LowerDst")
			continue
		}

		if err := node.doLowerSend(pkt); err != nil {
			node.Log().Printf("lowerSendRoutine: %s", err)
			continue
		}
	}
}

func (node *Node) doLowerSend(pkt rovy.Packet) error {
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

		if pkt.UpperDst.Empty() {
			node.Log().Printf("upperSendRoutine: packet without UpperDst")
			continue
		}

		if err := node.doUpperSend(pkt); err != nil {
			node.Log().Printf("upperSendRoutine: %s", err)
			continue
		}
	}
}

func (node *Node) doUpperSend(pkt rovy.Packet) error {
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
