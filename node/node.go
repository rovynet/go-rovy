package node

import (
	"fmt"
	"log"

	pretty "github.com/kr/pretty"

	multiaddr "github.com/multiformats/go-multiaddr"
	multiaddrnet "github.com/multiformats/go-multiaddr/net"
	varint "github.com/multiformats/go-varint"
	rovy "pkt.dev/go-rovy"
	forwarder "pkt.dev/go-rovy/forwarder"
	multigram "pkt.dev/go-rovy/multigram"
	routing "pkt.dev/go-rovy/routing"
	session "pkt.dev/go-rovy/session"
)

type Listener multiaddrnet.PacketConn

type UpperHandler func(rovy.UpperPacket) error

// TODO: move lower connection stuff to a Peering type (Connect, SendLower, Handle*)
type Node struct {
	peerid    rovy.PeerID
	logger    *log.Logger
	listeners []Listener
	sessions  *session.SessionManager
	handlers  map[uint64]UpperHandler
	forwarder *forwarder.Forwarder
	routing   *routing.Routing
	RxTpt     uint64
	RxLower   uint64
	RxUpper   uint64
}

func NewNode(privkey rovy.PrivateKey, logger *log.Logger) *Node {
	pubkey := privkey.PublicKey()
	peerid := rovy.NewPeerID(pubkey)

	node := &Node{
		peerid:   peerid,
		logger:   logger,
		handlers: map[uint64]UpperHandler{},
		routing:  routing.NewRouting(logger),
	}

	node.sessions = session.NewSessionManager(privkey, logger, node.ConnectedLower)

	node.forwarder = forwarder.NewForwarder(node.sessions.Multigram(), logger)

	node.forwarder.Attach(peerid, func(lpkt rovy.LowerPacket) error {
		upkt := rovy.NewUpperPacket(lpkt.Packet)
		return node.ReceiveUpper(upkt)
	})

	return node
}

func (node *Node) PeerID() rovy.PeerID {
	return node.peerid
}

func (node *Node) Log() *log.Logger {
	return node.logger
}

func (node *Node) Multigram() *multigram.Table {
	return node.SessionManager().Multigram()
}

func (node *Node) Adresses() (addrs []multiaddr.Multiaddr) {
	for _, lis := range node.listeners {
		addrs = append(addrs, lis.LocalMultiaddr())
	}
	return addrs
}

func (node *Node) SessionManager() *session.SessionManager {
	return node.sessions
}

func (node *Node) Forwarder() *forwarder.Forwarder {
	return node.forwarder
}

func (node *Node) Routing() *routing.Routing {
	return node.routing
}

// XXX this shouldn't & mustn't be triggered for Upper sessions,
//     otherwise forwarder.Attach deadlocks.
func (node *Node) ConnectedLower(peerid rovy.PeerID) {
	send := func(pkt rovy.LowerPacket) error {
		pkt.LowerDst = peerid
		return node.SendLower(pkt)
	}

	slot, err := node.forwarder.Attach(peerid, send)
	if err != nil {
		node.Log().Printf("connected to %s, but forwarder error: %s", peerid, err)
		return
	}

	node.routing.AddRoute(peerid, slot)

	node.Log().Printf("connected to %s", peerid)
}

func (node *Node) Listen(lisaddr multiaddr.Multiaddr) error {
	pktconn, err := multiaddrnet.ListenPacket(lisaddr)
	if err != nil {
		return err
	}
	node.listeners = append(node.listeners, pktconn)

	go func() {
		for {
			pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))

			n, raddr, err := pktconn.ReadFrom(pkt.Bytes())
			if err != nil {
				node.logger.Printf("ReadFrom: %s", err)
				continue
			}

			node.RxTpt += 1

			pkt.Length = n
			pkt.TptSrc, _ = multiaddrnet.FromNetAddr(raddr) // TODO handle error

			if err = node.ReceiveLower(pkt); err != nil {
				node.logger.Printf("Listen: loop: %s", err)
			}
		}
	}()

	return nil
}

func (node *Node) Handle(codec uint64, cb UpperHandler) {
	_, present := node.handlers[codec]
	if present {
		return
	}

	node.sessions.Multigram().AddCodec(codec)
	node.handlers[codec] = cb
}

// TODO: timeout
// TODO: check if we already have a session
func (node *Node) Connect(peerid rovy.PeerID, raddr multiaddr.Multiaddr) error {
	pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))

	if raddr != nil { // aka ConnectLower
		hellopkt := session.NewHelloPacket(pkt, rovy.LowerOffset, rovy.LowerPadding)
		hellopkt, err := node.SessionManager().CreateHello(hellopkt, peerid, raddr)
		if err != nil {
			return err
		}

		hellopkt.TptDst = raddr
		err = node.sendTransport(hellopkt.Packet)
		if err != nil {
			node.logger.Printf("Connect: sendTransport: %s", err)
			return err
		}
	} else { // aka ConnectUpper
		route, err := node.Routing().GetRoute(peerid)
		if err != nil {
			return err
		}

		hellopkt := session.NewHelloPacket(pkt, rovy.UpperOffset, rovy.UpperPadding)
		hellopkt, err = node.SessionManager().CreateHello(hellopkt, peerid, raddr)
		if err != nil {
			return err
		}

		upkt := rovy.NewUpperPacket(hellopkt.Packet)
		upkt.SetRoute(route)

		err = node.Forwarder().SendPacket(upkt)
		if err != nil {
			return fmt.Errorf("Connect: forwarder: %s", err)
		}
	}

	if err := <-node.sessions.WaitFor(peerid); err != nil {
		node.Log().Printf("connect %s: %s", peerid, err)
		return err
	}

	return nil
}

func (node *Node) SendLower(pkt rovy.LowerPacket) error {
	datapkt := session.NewDataPacket(pkt.Packet, rovy.LowerOffset, rovy.LowerPadding)

	raddr, err := node.SessionManager().CreateData(datapkt, datapkt.LowerDst)
	if err != nil {
		return err
	}

	datapkt.TptDst = raddr
	return node.sendTransport(datapkt.Packet)
}

func (node *Node) sendTransport(pkt rovy.Packet) error {
	_, err := node.listeners[0].WriteToMultiaddr(pkt.Bytes(), pkt.TptDst)
	return err
}

func (node *Node) ReceiveLower(pkt rovy.Packet) error {
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
		resppkt, err := node.SessionManager().HandleResponse(resppkt, pkt.TptSrc)
		if err != nil {
			return err
		}
		return nil

	case session.DataMsgType:
		datapkt := session.NewDataPacket(pkt, rovy.LowerOffset, rovy.LowerPadding)

		peerid, err := node.SessionManager().HandleData(datapkt)
		if err != nil {
			return err
		}

		lowpkt := rovy.NewLowerPacket(datapkt.Packet)
		lowpkt.Length = datapkt.Length
		lowpkt.LowerSrc = peerid

		codec, err := lowpkt.Codec()
		if err != nil {
			return fmt.Errorf("ReceiveLower: codec: %s", err)
		}

		node.RxLower += 1

		switch node.SessionManager().Multigram().LookupCodec(codec) {
		case forwarder.DataMulticodec:
			return node.Forwarder().HandlePacket(lowpkt)
		}

		// TODO should have a codec for this double-encryption-avoidance hack
		upkt := rovy.NewUpperPacket(lowpkt.Packet)
		upkt.UpperSrc = upkt.LowerSrc
		return node.ReceiveUpperDirect(upkt)
	}

	return fmt.Errorf("ReceiveLower: dropping packet with unknown MsgType 0x%x", msgtype)
}

func (node *Node) Send(to rovy.PeerID, codec uint64, p []byte) error {
	route, err := node.Routing().GetRoute(to)
	if err != nil {
		return err
	}

	pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))
	upkt := rovy.NewUpperPacket(pkt)
	upkt.UpperDst = to
	upkt.SetCodec(node.SessionManager().Multigram().LookupNumber(codec))
	upkt.SetRoute(route)
	upkt = upkt.SetPayload(p)

	return node.SendUpper(upkt)
}

func (node *Node) SendUpper(upkt rovy.UpperPacket) error {
	upkt.UpperSrc = node.PeerID()

	// XXX not sure what the relationship is with handshake packets and non-data packets
	if upkt.RouteLen() == forwarder.HopLength {
		lpkt := rovy.NewLowerPacket(upkt.Packet)
		lpkt.LowerSrc = node.PeerID()
		lpkt.LowerDst = upkt.UpperDst
		return node.SendLower(lpkt)
	}

	datapkt := session.NewDataPacket(upkt.Packet, rovy.UpperOffset, rovy.UpperPadding)
	_, err := node.SessionManager().CreateData(datapkt, upkt.UpperDst)
	if err != nil {
		return err
	}

	return node.Forwarder().SendPacket(upkt)
}

func (node *Node) ReceiveUpperDirect(upkt rovy.UpperPacket) error {
	// node.Log().Printf("ReceiveUpperDirect: via=%s route=%s length=%d payload=%#v packet=%# v", upkt.LowerSrc, upkt.Route(), len(upkt.Payload()), upkt.Payload(), pretty.Formatter(upkt))

	node.RxUpper += 1

	number, err := upkt.Codec()
	if err != nil {
		return err
	}

	mgram := node.SessionManager().Multigram()
	codec := mgram.LookupCodec(number)

	cb, present := node.handlers[codec]
	if !present {
		node.logger.Printf("ReceiveUpperDirect: dropping packet with unknown codec %d (number=%d) multigram=%# v", codec, number, pretty.Formatter(mgram))
		return err
	}
	return cb(upkt)
}

func (node *Node) ReceiveUpper(upkt rovy.UpperPacket) error {
	// node.logger.Printf("ReceiveUpper: via=%s route=%s length=%d payload=%#v", upkt.LowerSrc, upkt.Route(), len(upkt.Payload()), upkt.Payload())

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

		err = node.forwarder.SendPacket(upkt2)
		if err != nil {
			return fmt.Errorf("ReceiveUpper: %s", err)
		}

		return nil

	case session.ResponseMsgType:
		resppkt := session.NewResponsePacket(upkt.Packet, rovy.UpperOffset, rovy.UpperPadding)
		resppkt, err := node.SessionManager().HandleResponse(resppkt, nil)
		if err != nil {
			return err
		}
		return nil

	case session.DataMsgType:
		datapkt := session.NewDataPacket(upkt.Packet, rovy.UpperOffset, rovy.UpperPadding)

		peerid, err := node.SessionManager().HandleData(datapkt)
		if err != nil {
			return err
		}
		datapkt.UpperSrc = peerid

		// node.logger.Printf("ReceiveUpper: datapkt=%#v", datapkt)

		node.Routing().AddRoute(datapkt.UpperSrc, upkt.Route().Reverse()) // XXX slowness

		upkt := rovy.NewUpperPacket(datapkt.Packet)
		return node.ReceiveUpperDirect(upkt)

	case session.PlaintextMsgType:
		return node.ReceiveUpperPlaintext(upkt.Bytes()[rovy.UpperOffset:], upkt.Route())
	}

	return fmt.Errorf("ReceiveUpper: dropping packet with unknown MsgType 0x%x", msgtype)
}

// TODO actually sign the thing
func (node *Node) SendPlaintext(route rovy.Route, codec uint64, b []byte) error {
	// hdr := varint.ToUvarint(codec)
	// b = append([]byte{0x0, 0x0, 0x0, 0x0}, b...) // XXX slowness
	// copy(b[0:4], hdr)

	pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))
	ptpkt := session.NewPlaintextPacket(pkt, rovy.UpperOffset, rovy.UpperPadding)
	ptpkt = ptpkt.SetPlaintext(b)
	ptpkt.SetSender(node.PeerID().PublicKey())

	// fwdbuf := ptpkt.Bytes()[rovy.FwdOffset:]
	// node.Log().Printf("SendPlaintext: fwdbuf=%#v", fwdbuf)
	// return node.forwarder.SendPacket(fwdbuf, node.PeerID(), route)

	upkt := rovy.NewUpperPacket(rovy.NewPacket(make([]byte, rovy.TptMTU)))
	upkt.UpperSrc = node.PeerID()
	// upkt.SetMsgType(session.DataMsgType)
	upkt.SetCodec(codec)
	upkt.SetRoute(route)
	upkt = upkt.SetPayload(ptpkt.Plaintext())

	node.Log().Printf("SendPlaintext: %# v", pretty.Formatter(upkt))
	return node.Forwarder().SendPacket(upkt)
}

// TODO actually verify signature
func (node *Node) ReceiveUpperPlaintext(b []byte, route rovy.Route) error {
	pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))
	ptpkt := session.NewPlaintextPacket(pkt, rovy.UpperOffset, rovy.UpperPadding)
	copy(ptpkt.Bytes()[rovy.UpperOffset:], b)
	ptpkt.Length = rovy.UpperOffset + len(b)

	pt := ptpkt.Plaintext()

	// ptpkt.Route = route

	codec, _, err := varint.FromUvarint(pt[0:4])
	if err != nil {
		return err
	}

	node.Log().Printf("ReceiveUpperPlaintext: got %#v", pt)

	cb, present := node.handlers[codec]
	if !present {
		return fmt.Errorf("ReceiveUpperPlaintext: dropping packet with unknown codec %d", codec)
	}

	// XXX fucking horrific. this whole plaintext business must move to forwarder/
	upkt := rovy.NewUpperPacket(rovy.NewPacket(make([]byte, rovy.TptMTU)))
	upkt.UpperSrc = rovy.NewPeerID(ptpkt.Sender())
	upkt.SetMsgType(session.DataMsgType)
	upkt.SetRoute(route)
	upkt.SetCodec(node.SessionManager().Multigram().LookupNumber(codec))
	upkt = upkt.SetPayload(pt)

	// return cb(pt[4:], rovy.NewPeerID(ptpkt.Sender()), route)
	return cb(upkt)
}
