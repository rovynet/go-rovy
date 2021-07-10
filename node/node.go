package node

import (
	"fmt"
	"log"

	multiaddr "github.com/multiformats/go-multiaddr"
	multiaddrnet "github.com/multiformats/go-multiaddr/net"
	varint "github.com/multiformats/go-varint"
	rovy "pkt.dev/go-rovy"
	forwarder "pkt.dev/go-rovy/forwarder"
	routing "pkt.dev/go-rovy/routing"
	session "pkt.dev/go-rovy/session"
)

type Listener multiaddrnet.PacketConn

type DataHandler func([]byte, rovy.PeerID, rovy.Route) error

// TODO: move lower connection stuff to a Peering type (Connect, SendLower, Handle*)
type Node struct {
	peerid    rovy.PeerID
	logger    *log.Logger
	listeners []Listener
	sessions  *session.SessionManager
	handlers  map[uint64]DataHandler
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
		handlers: map[uint64]DataHandler{},
		routing:  routing.NewRouting(logger),
	}

	node.sessions = session.NewSessionManager(privkey, logger, node.ConnectedLower)

	node.forwarder = forwarder.NewForwarder(node.sessions.Multigram(), logger)

	node.forwarder.Attach(peerid, func(peerid rovy.PeerID, p []byte) error {
		rlen := p[5]
		route := rovy.NewRoute(p[6 : 6+rlen]...).Reverse()
		data := p[20:]
		return node.ReceiveUpper(peerid, data, route)
	})

	return node
}

func (node *Node) PeerID() rovy.PeerID {
	return node.peerid
}

func (node *Node) Log() *log.Logger {
	return node.logger
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
	send := func(from rovy.PeerID, buf []byte) error {
		// node.Log().Printf("ConnectedLower: forwarding %d bytes to %s: %#v", len(buf), peerid, buf)
		return node.SendLower(peerid, buf)
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
				node.logger.Printf("ReceiveLower: %s", err)
			}
		}
	}()

	return nil
}

func (node *Node) Handle(codec uint64, cb DataHandler) {
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
		hellopkt := session.NewHelloPacket(pkt, rovy.LowerOffset)
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

		hellopkt := session.NewHelloPacket(pkt, rovy.UpperOffset)
		hellopkt, err = node.SessionManager().CreateHello(hellopkt, peerid, raddr)
		if err != nil {
			return err
		}

		fwdbuf := hellopkt.Bytes()[rovy.FwdOffset:]
		err = node.forwarder.SendPacket(fwdbuf, node.PeerID(), route)
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

func (node *Node) SendLower(pid rovy.PeerID, p []byte) error {
	pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))
	datapkt := session.NewDataPacket(pkt, 0)
	datapkt = datapkt.SetPlaintext(p)

	raddr, err := node.SessionManager().CreateData(datapkt, pid)
	if err != nil {
		return err
	}

	datapkt.TptDst = raddr
	return node.sendTransport(datapkt.Packet)
}

func (node *Node) sendTransport(pkt rovy.Packet) error {
	// node.Log().Printf("sendTransport: %#v", pkt.Bytes())
	_, err := node.listeners[0].WriteToMultiaddr(pkt.Bytes(), pkt.TptDst)
	return err
}

func (node *Node) ReceiveLower(pkt rovy.Packet) error {
	// node.Log().Printf("ReceiveLower: %#v", pkt.Bytes())
	msgtype := pkt.MsgType()
	switch msgtype {
	case session.HelloMsgType:
		hellopkt := session.NewHelloPacket(pkt, rovy.LowerOffset)

		resppkt, err := node.SessionManager().HandleHello(hellopkt, pkt.TptSrc)
		if err != nil {
			return err
		}

		resppkt.TptDst = hellopkt.TptSrc
		return node.sendTransport(resppkt.Packet)
	case session.ResponseMsgType:
		resppkt := session.NewResponsePacket(pkt, rovy.LowerOffset)
		resppkt, err := node.SessionManager().HandleResponse(resppkt, pkt.TptSrc)
		if err != nil {
			return err
		}
		return nil
	case session.DataMsgType:
		datapkt := session.NewDataPacket(pkt, rovy.LowerOffset)

		peerid, err := node.SessionManager().HandleData(datapkt)
		if err != nil {
			return err
		}
		datapkt.LowerSrc = peerid

		data := datapkt.Plaintext()

		codec, _, err := node.sessions.Multigram().FromUvarint(data) // XXX LowerPacket
		if err != nil {
			return err
		}

		node.RxLower += 1

		switch codec {
		case forwarder.DataMulticodec:
			return node.forwarder.HandlePacket(data, peerid)
		case forwarder.ErrorMulticodec:
			return node.forwarder.HandleError(data, peerid)
		}

		return node.ReceiveUpperDirect(peerid, data, rovy.EmptyRoute)
	}

	return fmt.Errorf("ReceiveLower: dropping packet with unknown MsgType 0x%x", msgtype)
}

func (node *Node) Send(peerid rovy.PeerID, codec uint64, p []byte) error {
	route, err := node.Routing().GetRoute(peerid)
	if err != nil {
		return err
	}

	return node.SendUpper(peerid, codec, p, route)
}

func (node *Node) SendUpper(peerid rovy.PeerID, codec uint64, p []byte, route rovy.Route) error {
	hdr := node.sessions.Multigram().ToUvarint(codec)
	p = append([]byte{0x0, 0x0, 0x0, 0x0}, p...) // XXX slowness
	copy(p[0:4], hdr)

	if route.Len() == forwarder.HopLength {
		return node.SendLower(peerid, p)
	}

	pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))
	datapkt := session.NewDataPacket(pkt, rovy.UpperOffset)
	datapkt = datapkt.SetPlaintext(p)

	_, err := node.SessionManager().CreateData(datapkt, peerid)
	if err != nil {
		return err
	}

	fwdbuf := datapkt.Bytes()[rovy.FwdOffset:]
	// node.Log().Printf("SendUpper: fwdbuf=#%v", fwdbuf)
	return node.forwarder.SendPacket(fwdbuf, node.PeerID(), route)
}

func (node *Node) ReceiveUpperDirect(from rovy.PeerID, data []byte, route rovy.Route) error {
	// node.Log().Printf("ReceiveUpperDirect: from %s along %s: %#v", from, route, data)

	sess, _, present := node.SessionManager().Find(from)
	if !present {
		node.logger.Printf("lost track of session while handling packet from %s", from)
		// return nil // XXX return error instead?
		return fmt.Errorf("lost track of session: %s", from)
	}

	node.RxUpper += 1

	codec, _, err := sess.RemoteMultigram().FromUvarint(data[0:4])
	if err != nil {
		return err
	}

	cb, present := node.handlers[codec]
	if !present {
		node.logger.Printf("ReceiveUpperDirect: dropping packet with unknown codec %d", codec)
		return err
	}
	return cb(data[4:], from, route)
}

func (node *Node) ReceiveUpper(from rovy.PeerID, b []byte, route rovy.Route) error {
	// node.logger.Printf("ReceiveUpper: packet=%#v", b)

	pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))
	pkt.Length = rovy.UpperOffset + len(b)
	copy(pkt.Bytes()[rovy.UpperOffset:], b)

	pkt.LowerSrc = from
	// pkt.Route = route

	switch b[0] {
	case session.HelloMsgType:
		hellopkt := session.NewHelloPacket(pkt, rovy.UpperOffset)

		resppkt, err := node.SessionManager().HandleHello(hellopkt, nil)
		if err != nil {
			return err
		}
		resppkt.UpperDst = hellopkt.UpperSrc
		// resppkt.Route = hellopkt.Route

		respbuf := resppkt.Bytes()[rovy.FwdOffset:]

		return node.forwarder.SendPacket(respbuf, resppkt.UpperDst, route)

	case session.ResponseMsgType:
		resppkt := session.NewResponsePacket(pkt, rovy.UpperOffset)
		resppkt, err := node.SessionManager().HandleResponse(resppkt, nil)
		if err != nil {
			return err
		}
		return nil

	case session.DataMsgType:
		datapkt := session.NewDataPacket(pkt, rovy.UpperOffset)

		peerid, err := node.SessionManager().HandleData(datapkt)
		if err != nil {
			return err
		}
		datapkt.UpperSrc = peerid

		// node.logger.Printf("ReceiveUpper: datapkt=%#v", datapkt)

		node.Routing().AddRoute(peerid, route) // XXX slowness

		return node.ReceiveUpperDirect(peerid, datapkt.Plaintext(), route)

	case session.PlaintextMsgType:
		return node.ReceiveUpperPlaintext(pkt.Bytes()[rovy.UpperOffset:], route)
	}

	return fmt.Errorf("ReceiveUpper: dropping packet with unknown MsgType 0x%x", b[0])
}

// TODO actually sign the thing
func (node *Node) SendPlaintext(route rovy.Route, codec uint64, b []byte) error {
	hdr := varint.ToUvarint(codec)
	b = append([]byte{0x0, 0x0, 0x0, 0x0}, b...) // XXX slowness
	copy(b[0:4], hdr)

	pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))
	ptpkt := session.NewPlaintextPacket(pkt, rovy.UpperOffset)
	ptpkt = ptpkt.SetPlaintext(b)
	ptpkt.SetSender(node.PeerID().PublicKey())

	fwdbuf := ptpkt.Bytes()[rovy.FwdOffset:]
	// node.Log().Printf("SendPlaintext: fwdbuf=%#v", fwdbuf)
	return node.forwarder.SendPacket(fwdbuf, node.PeerID(), route)
}

// TODO actually verify signature
func (node *Node) ReceiveUpperPlaintext(b []byte, route rovy.Route) error {
	pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))
	ptpkt := session.NewPlaintextPacket(pkt, rovy.UpperOffset)
	copy(ptpkt.Bytes()[rovy.UpperOffset:], b)
	ptpkt.Length = rovy.UpperOffset + len(b)

	pt := ptpkt.Plaintext()

	// ptpkt.Route = route

	codec, _, err := varint.FromUvarint(pt[0:4])
	if err != nil {
		return err
	}

	// node.Log().Printf("ReceiveUpperPlaintext: got %#v", pt)

	cb, present := node.handlers[codec]
	if !present {
		node.logger.Printf("ReceiveUpperPlaintext: dropping packet with unknown codec %d", codec)
		return err
	}

	return cb(pt[4:], rovy.NewPeerID(ptpkt.Sender()), route)
}
