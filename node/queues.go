package node

import (
	"fmt"

	rovy "pkt.dev/go-rovy"
	forwarder "pkt.dev/go-rovy/forwarder"
	session "pkt.dev/go-rovy/session"
)

func (node *Node) lowerHelloSendRoutine() {
	for {
		pkt := node.lowerHelloSendQ.Get()

		if pkt.TptDst == nil {
			node.Log().Printf("lowerHelloSendRoutine: packet without TptDst")
			continue
		}
		if pkt.LowerDst.Empty() {
			node.Log().Printf("lowerHelloSendRoutine: packet without LowerDst")
			continue
		}

		if err := node.doLowerHelloSend(pkt); err != nil {
			node.Log().Printf("lowerHelloSendRoutine: %s", err)
			continue
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

// XXX: requires pkt.TptSrc
func (node *Node) lowerHelloRecvRoutine() {
	for {
		pkt := node.lowerHelloRecvQ.Get()

		if pkt.TptSrc == nil {
			node.Log().Printf("lowerHelloRecvRoutine: dropping packet without TptSrc")
			continue
		}

		if err := node.doLowerHelloRecv(pkt); err != nil {
			node.Log().Printf("lowerHelloRecvRoutine: %s", err)
			continue
		}
	}
}

func (node *Node) doLowerHelloRecv(pkt rovy.Packet) error {
	msgtype := pkt.MsgType()
	switch msgtype {
	case session.HelloMsgType:
		hellopkt := session.NewHelloPacket(pkt, rovy.LowerOffset, rovy.LowerPadding)
		resppkt, err := node.SessionManager().HandleHello(hellopkt, pkt.TptSrc)
		if err != nil {
			return err
		}
		resppkt.TptDst = hellopkt.TptSrc
		node.sendTransport(resppkt.Packet)
	case session.ResponseMsgType:
		resppkt := session.NewResponsePacket(pkt, rovy.LowerOffset, rovy.LowerPadding)
		resppkt, peerid, err := node.SessionManager().HandleResponse(resppkt, pkt.TptSrc)
		if err != nil {
			return err
		}
		node.connectedCallback(peerid, true)
	default:
		return fmt.Errorf("dropping packet with unknown MsgType 0x%x", msgtype)
	}
	return nil
}

func (node *Node) lowerUnsealRoutine() {
	for {
		pkt := node.lowerUnsealQ.Get()

		msgtype := pkt.MsgType()
		switch msgtype {
		case session.DataMsgType:
			err := node.doLowerUnseal(pkt)
			if err != nil {
				node.Log().Printf("lowerUnsealRoutine: %s", err)
				continue
			}
		case session.HelloMsgType, session.ResponseMsgType:
			node.lowerHelloRecvQ.Put(pkt)
		default:
			node.Log().Printf("lowerUnsealRoutine: dropping packet with unknown MsgType 0x%x", msgtype)
		}
	}
}

func (node *Node) doLowerUnseal(pkt rovy.Packet) error {
	datapkt := session.NewDataPacket(pkt, rovy.LowerOffset, rovy.LowerPadding)

	peerid, firstdata, err := node.SessionManager().HandleData(datapkt)
	if err != nil {
		return err
	}

	if firstdata {
		node.connectedCallback(peerid, true)
	}

	node.RxLower += 1

	datapkt.LowerSrc = peerid
	node.lowerMuxQ.Put(datapkt.Packet)
	return nil
}

func (node *Node) lowerSealRoutine() {
	for {
		pkt := node.lowerSealQ.Get()

		if pkt.LowerDst.Empty() {
			node.Log().Printf("lowerSealRoutine: dropping packet without LowerDst")
			continue
		}

		if err := node.doLowerSeal(pkt); err != nil {
			node.Log().Printf("lowerSealRoutine: %s", err)
			continue
		}
	}
}

func (node *Node) doLowerSeal(pkt rovy.Packet) error {
	datapkt := session.NewDataPacket(pkt, rovy.LowerOffset, rovy.LowerPadding)

	raddr, err := node.SessionManager().CreateData(datapkt, datapkt.LowerDst)
	if err != nil {
		return err
	}

	datapkt.TptDst = raddr
	return node.sendTransport(datapkt.Packet)
}

func (node *Node) lowerMuxRoutine() {
	for {
		pkt := node.lowerMuxQ.Get()

		if pkt.LowerSrc.Empty() {
			node.Log().Printf("lowerMuxRoutine: dropping packet without LowerSrc")
			continue
		}

		if err := node.doLowerMux(pkt); err != nil {
			node.Log().Printf("lowerMuxRoutine: %s", err)
			continue
		}
	}
}

func (node *Node) doLowerMux(pkt rovy.Packet) error {
	lowpkt := rovy.NewLowerPacket(pkt)

	codec, err := lowpkt.Codec()
	if err != nil {
		return fmt.Errorf("codec: %s", err)
	}

	if codec == forwarder.DataMulticodec {
		return node.Forwarder().HandlePacket(lowpkt)
	}

	if codec == DirectUpperCodec {
		lowpkt.UpperSrc = pkt.LowerSrc
		node.upperMuxQ.Put(lowpkt.Packet)
		return nil
	}

	cb, present := node.lowerHandlers[codec]
	if !present {
		return fmt.Errorf("dropping packet with unknown lower codec 0x%x from %s", codec, lowpkt.LowerSrc)
	}

	return cb(lowpkt)
}

// upper unseal

func (node *Node) upperUnsealRoutine() {
	for {
		pkt := node.upperUnsealQ.Get()

		msgtype := pkt.Buf[rovy.UpperOffset]
		switch msgtype {
		case session.DataMsgType:
			err := node.doUpperUnseal(pkt)
			if err != nil {
				node.Log().Printf("upperUnsealRoutine: %s", err)
				continue
			}
		case session.HelloMsgType, session.ResponseMsgType:
			node.upperHelloRecvQ.Put(pkt)
		default:
			node.Log().Printf("upperUnsealRoutine: dropping packet with unknown MsgType 0x%x", msgtype)
		}
	}
}

func (node *Node) doUpperUnseal(pkt rovy.Packet) error {
	upkt := rovy.NewUpperPacket(pkt)
	datapkt := session.NewDataPacket(upkt.Packet, rovy.UpperOffset, rovy.UpperPadding)

	peerid, firstdata, err := node.SessionManager().HandleData(datapkt)
	if err != nil {
		return err
	}

	if firstdata {
		node.connectedCallback(peerid, false)
	}

	datapkt.UpperSrc = peerid
	node.Routing().AddRoute(datapkt.UpperSrc, upkt.Route().Reverse()) // XXX slowness

	node.upperMuxQ.Put(datapkt.Packet)
	return nil
}

// upper hello recv

func (node *Node) upperHelloRecvRoutine() {
	for {
		pkt := node.upperHelloRecvQ.Get()

		if err := node.doUpperHelloRecv(pkt); err != nil {
			node.Log().Printf("upperHelloRecvRoutine: %s", err)
			continue
		}
	}
}

func (node *Node) doUpperHelloRecv(pkt rovy.Packet) error {
	upkt := rovy.NewUpperPacket(pkt)

	msgtype := upkt.Buf[rovy.UpperOffset]
	switch msgtype {
	case session.HelloMsgType:
		hellopkt := session.NewHelloPacket(upkt.Packet, rovy.UpperOffset, rovy.UpperPadding)

		resppkt, err := node.SessionManager().HandleHello(hellopkt, nil)
		if err != nil {
			return err
		}

		upkt2 := rovy.NewUpperPacket(resppkt.Packet)
		upkt2.SetRoute(upkt.Route().Reverse())
		upkt2.LowerSrc = node.PeerID()

		if err = node.forwarder.SendPacket(upkt2); err != nil {
			return fmt.Errorf("forwarder: %s", err)
		}
		return nil
	case session.ResponseMsgType:
		resppkt := session.NewResponsePacket(upkt.Packet, rovy.UpperOffset, rovy.UpperPadding)
		resppkt, peerid, err := node.SessionManager().HandleResponse(resppkt, nil)
		if err != nil {
			return err
		}

		node.connectedCallback(peerid, false)
		return nil
	default:
		return fmt.Errorf("dropping packet with unknown MsgType 0x%x", msgtype)
	}
}

// upper mux

func (node *Node) upperMuxRoutine() {
	for {
		pkt := node.upperMuxQ.Get()

		if err := node.doUpperMux(pkt); err != nil {
			node.Log().Printf("upperMuxRoutine: %s", err)
			continue
		}
	}
}

func (node *Node) doUpperMux(pkt rovy.Packet) error {
	node.RxUpper += 1

	upkt := rovy.NewUpperPacket(pkt)

	codec, err := upkt.Codec()
	if err != nil {
		return fmt.Errorf("codec: %s", err)
	}

	cb, present := node.upperHandlers[codec]
	if !present {
		return fmt.Errorf("dropping packet with unknown upper codec 0x%x from %s", codec, upkt.LowerSrc)
	}

	return cb(upkt)
}

// upper seal

func (node *Node) upperSealRoutine() {
	for {
		pkt := node.upperSealQ.Get()

		if pkt.UpperDst.Empty() {
			node.Log().Printf("upperSealRoutine: packet without UpperDst")
			continue
		}

		if err := node.doUpperSeal(pkt); err != nil {
			node.Log().Printf("upperSealRoutine: %s", err)
			continue
		}
	}
}

func (node *Node) doUpperSeal(pkt rovy.Packet) error {
	upkt := rovy.NewUpperPacket(pkt)

	if upkt.RouteLen() == forwarder.HopLength {
		lpkt := rovy.NewLowerPacket(upkt.Packet)
		lpkt.SetCodec(DirectUpperCodec)
		lpkt.LowerDst = upkt.UpperDst
		node.lowerSealQ.PutWithBackpressure(lpkt.Packet)
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

// upper hello send

func (node *Node) upperHelloSendRoutine() {
	for {
		pkt := node.upperHelloSendQ.Get()

		if pkt.UpperDst.Empty() {
			node.Log().Printf("upperHelloSendRoutine: packet without UpperDst")
			continue
		}

		if err := node.doUpperHelloSend(pkt); err != nil {
			node.Log().Printf("upperHelloSendRoutine: %s", err)
			continue
		}
	}
}

func (node *Node) doUpperHelloSend(pkt rovy.Packet) error {
	hellopkt := session.NewHelloPacket(pkt, rovy.UpperOffset, rovy.UpperPadding)
	hellopkt, err := node.SessionManager().CreateHello(hellopkt, pkt.UpperDst, nil)
	if err != nil {
		return err
	}

	// TODO: route lookup should move to caller
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
