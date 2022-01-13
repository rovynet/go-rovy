package node

import (
	"fmt"

	rovy "go.rovy.net"
	forwarder "go.rovy.net/forwarder"
	session "go.rovy.net/session"
)

// hello receive

func (node *Node) helloRecvRoutine() {
	for {
		pkt := node.helloRecvQ.Get()

		var fn func(rovy.Packet) error

		// Packet only has LowerSrc if it was a data packet during the lower phase.
		// That means if LowerSrc is set, this is definitely not a lower hello.
		if pkt.LowerSrc.Empty() {
			fn = node.doLowerHelloRecv
		} else {
			fn = node.doUpperHelloRecv
		}

		err := fn(pkt)
		if err == ErrDontRelease {
			continue
		}
		if err != nil {
			node.Log().Printf("helloRecvRoutine: %s", err)
		}
		node.ReleasePacket(pkt)
	}
}

func (node *Node) doLowerHelloRecv(pkt rovy.Packet) error {
	if pkt.TptSrc.Empty() {
		return fmt.Errorf("lower packet without TptSrc")
	}

	msgtype := pkt.MsgType()
	switch msgtype {
	case session.HelloMsgType:
		hellopkt := session.NewHelloPacket(pkt, rovy.LowerOffset, rovy.LowerPadding)
		resppkt, err := node.SessionManager().HandleHello(hellopkt, pkt.TptSrc)
		if err != nil {
			return err
		}
		resppkt.TptDst = hellopkt.TptSrc
		return node.sendTransport(resppkt.Packet)
	case session.ResponseMsgType:
		resppkt := session.NewResponsePacket(pkt, rovy.LowerOffset, rovy.LowerPadding)
		resppkt, peerid, err := node.SessionManager().HandleResponse(resppkt, pkt.TptSrc)
		if err != nil {
			return err
		}
		node.connectedCallback(peerid, true)
		return nil
	default:
		return fmt.Errorf("packet with unknown MsgType 0x%x", msgtype)
	}
}

func (node *Node) doUpperHelloRecv(pkt rovy.Packet) error {
	upkt := rovy.NewUpperPacket(pkt)

	msgtype := upkt.Buf[rovy.UpperOffset]
	switch msgtype {
	case session.HelloMsgType:
		hellopkt := session.NewHelloPacket(upkt.Packet, rovy.UpperOffset, rovy.UpperPadding)

		resppkt, err := node.SessionManager().HandleHello(hellopkt, rovy.UDPMultiaddr{})
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
		resppkt, peerid, err := node.SessionManager().HandleResponse(resppkt, rovy.UDPMultiaddr{})
		if err != nil {
			return err
		}

		node.connectedCallback(peerid, false)
		return nil
	default:
		return fmt.Errorf("packet with unknown MsgType 0x%x", msgtype)
	}
}

// lower recv

func (node *Node) lowerRecvRoutine() {
	for {
		pkt := node.lowerRecvQ.Get()

		msgtype := pkt.MsgType()
		switch msgtype {
		case session.DataMsgType:
			err := node.doLowerRecv(pkt)
			if err == ErrDontRelease {
				continue
			}
			if err != nil {
				node.Log().Printf("lowerRecvRoutine: %s", err)
			}
			node.ReleasePacket(pkt)
		case session.HelloMsgType, session.ResponseMsgType:
			node.helloRecvQ.Put(pkt)
		default:
			node.Log().Printf("lowerRecvRoutine: packet with unknown MsgType 0x%x", msgtype)
			node.ReleasePacket(pkt)
		}
	}
}

func (node *Node) doLowerRecv(pkt rovy.Packet) error {
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
	return ErrDontRelease
}

// lower mux

func (node *Node) lowerMuxRoutine() {
	for {
		pkt := node.lowerMuxQ.Get()

		err := node.doLowerMux(pkt)
		if err == ErrDontRelease {
			continue
		}
		if err != nil {
			node.Log().Printf("lowerMuxRoutine: %s", err)
		}
		node.ReleasePacket(pkt)
	}
}

func (node *Node) doLowerMux(pkt rovy.Packet) error {
	if pkt.LowerSrc.Empty() {
		return fmt.Errorf("lowerMuxRoutine: dropping packet without LowerSrc")
	}

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

	if err := cb(lowpkt); err != nil {
		return err
	}
	return ErrDontRelease
}

// upper recv

func (node *Node) upperRecvRoutine() {
	for {
		pkt := node.upperRecvQ.Get()

		msgtype := pkt.Buf[rovy.UpperOffset]
		switch msgtype {
		case session.DataMsgType:
			err := node.doUpperRecv(pkt)
			if err == ErrDontRelease {
				continue
			}
			if err != nil {
				node.Log().Printf("upperRecvRoutine: %s", err)
			}
			node.ReleasePacket(pkt)
		case session.HelloMsgType, session.ResponseMsgType:
			node.helloRecvQ.Put(pkt)
		default:
			node.Log().Printf("upperRecvRoutine: dropping packet with unknown MsgType 0x%x", msgtype)
			node.ReleasePacket(pkt)
		}
	}
}

func (node *Node) doUpperRecv(pkt rovy.Packet) error {
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
	return ErrDontRelease
}

// upper mux

func (node *Node) upperMuxRoutine() {
	for {
		pkt := node.upperMuxQ.Get()

		err := node.doUpperMux(pkt)
		if err == ErrDontRelease {
			continue
		}
		if err != nil {
			node.Log().Printf("upperMuxRoutine: %s", err)
		}
		node.ReleasePacket(pkt)
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

	if err := cb(upkt); err != nil {
		return err
	}
	return ErrDontRelease
}
