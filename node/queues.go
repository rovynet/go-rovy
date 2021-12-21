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
		// node.upperMuxQ.Put(lowpkt.Packet)
		return node.ReceiveUpperDirect(rovy.NewUpperPacket(lowpkt.Packet))
	}

	cb, present := node.lowerHandlers[codec]
	if !present {
		return fmt.Errorf("dropping packet with unknown lower codec 0x%x from %s", codec, lowpkt.LowerSrc)
	}

	return cb(lowpkt)
}
