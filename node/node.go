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
		_, clen, err := varint.FromUvarint(p)
		if err != nil {
			return err
		}
		// XXX sender can crash us i guess :)
		llen := int(p[1+clen])
		if len(p) < 2+llen+clen {
			return fmt.Errorf("preReceiveUpper: malformed packet header, length mismatch")
		}

		route := rovy.NewRoute(p[2+clen : 2+llen+clen]...).Reverse()
		data := p[2+llen+clen:]
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
			var p [rovy.PreliminaryMTU]byte
			n, raddr, err := pktconn.ReadFrom(p[:])
			if err != nil {
				node.logger.Printf("ReadFrom: %s", err)
				continue
			}

			node.RxTpt += 1

			pkt := rovy.Packet{Bytes: p[:n]}
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

func (node *Node) handleDataPacket(p []byte, maddr multiaddr.Multiaddr) ([]byte, rovy.PeerID, error) {
	pkt := &session.DataPacket{}
	if err := pkt.UnmarshalBinary(p); err != nil {
		return nil, rovy.EmptyPeerID, err
	}

	data, peerid, err := node.sessions.HandleData(pkt, maddr)
	if err != nil {
		return nil, rovy.EmptyPeerID, err
	}

	return data, peerid, nil
}

func (node *Node) handleHelloPacket(p []byte, maddr multiaddr.Multiaddr) ([]byte, error) {
	pkt := &session.HelloPacket{}
	if err := pkt.UnmarshalBinary(p); err != nil {
		return nil, err
	}

	pkt2, err := node.sessions.HandleHello(pkt, maddr)
	if err != nil {
		return nil, err
	}

	buf, err := pkt2.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func (node *Node) handleResponsePacket(p []byte, maddr multiaddr.Multiaddr) error {
	pkt := &session.ResponsePacket{}

	if err := pkt.UnmarshalBinary(p); err != nil {
		return err
	}

	if err := node.sessions.HandleHelloResponse(pkt, maddr); err != nil {
		return err
	}

	return nil
}

// TODO: timeout
// TODO: check if we already have a session
func (node *Node) Connect(peerid rovy.PeerID, raddr multiaddr.Multiaddr) error {
	hello, err := node.sessions.CreateHello(peerid, raddr)
	if err != nil {
		return err
	}

	buf, err := hello.MarshalBinary()
	if err != nil {
		return err
	}

	if raddr != nil {
		pkt := rovy.Packet{TptDst: raddr, Bytes: buf}
		if err = node.sendTransport(pkt); err != nil {
			node.logger.Printf("Connect: sendTransport: %s", err)
			return err
		}
	} else {
		route, err := node.Routing().GetRoute(peerid)
		if err != nil {
			return err
		}

		if err = node.forwarder.SendPacket(buf, node.PeerID(), route); err != nil {
			node.logger.Printf("Connect: SendPacket: %s", err)
			return err
		}
	}

	if err = <-node.sessions.WaitFor(peerid); err != nil {
		node.Log().Printf("connect %s: %s", peerid, err)
		return err
	}

	return nil
}

func (node *Node) SendLower(pid rovy.PeerID, p []byte) error {
	oldpkt, raddr, err := node.sessions.CreateData(pid, p)
	if err != nil {
		return err
	}

	buf, err := oldpkt.MarshalBinary()
	if err != nil {
		return err
	}

	pkt := rovy.Packet{TptDst: raddr, Bytes: buf}
	return node.sendTransport(pkt)
}

func (node *Node) sendTransport(pkt rovy.Packet) error {
	_, err := node.listeners[0].WriteToMultiaddr(pkt.Bytes, pkt.TptDst)
	return err
}

func (node *Node) ReceiveLower(pkt rovy.Packet) error {
	msgtype := pkt.MsgType()
	switch msgtype {
	case session.HelloMsgType:
		buf, err := node.handleHelloPacket(pkt.Bytes, pkt.TptSrc)
		if err != nil {
			return err
		}

		pkt2 := rovy.Packet{TptDst: pkt.TptSrc, Bytes: buf}
		return node.sendTransport(pkt2)
	case session.HelloResponseMsgType:
		return node.handleResponsePacket(pkt.Bytes, pkt.TptSrc)
	case session.DataMsgType:
		data, peerid, err := node.handleDataPacket(pkt.Bytes, pkt.TptSrc)
		if err != nil {
			return err
		}

		codec, _, err := node.sessions.Multigram().FromUvarint(data)
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
	p = append(hdr, p...) // XXX slowness

	if route.Len() == forwarder.HopLength {
		return node.SendLower(peerid, p)
	}

	pkt, _, err := node.sessions.CreateData(peerid, p)
	if err != nil {
		return err
	}

	buf, err := pkt.MarshalBinary()
	if err != nil {
		return err
	}

	return node.forwarder.SendPacket(buf, node.PeerID(), route)
}

func (node *Node) ReceiveUpperDirect(from rovy.PeerID, data []byte, route rovy.Route) error {
	sess, _, present := node.SessionManager().Find(from)
	if !present {
		node.logger.Printf("lost track of session while handling packet from %s", from)
		return nil // XXX return error instead?
	}

	node.RxUpper += 1

	codec, n, err := sess.Multigram().FromUvarint(data)
	if err != nil {
		return err
	}

	cb, present := node.handlers[codec]
	if !present {
		node.logger.Printf("ReceiveUpperDirect: dropping packet with unknown codec %d", codec)
		return err
	}

	return cb(data[n:], from, route)
}

func (node *Node) ReceiveUpper(from rovy.PeerID, b []byte, route rovy.Route) error {
	switch b[0] {
	case session.HelloMsgType:
		buf, err := node.handleHelloPacket(b, nil)
		if err != nil {
			return err
		}

		return node.forwarder.SendPacket(buf, from, route)
	case session.HelloResponseMsgType:
		return node.handleResponsePacket(b, nil)
	case session.DataMsgType:
		data, peerid, err := node.handleDataPacket(b, nil)
		if err != nil {
			return err
		}

		node.Routing().AddRoute(peerid, route) // XXX slowness

		return node.ReceiveUpperDirect(peerid, data, route)
	case session.PlaintextMsgType:
		return node.ReceiveUpperPlaintext(b, route)
	}

	return fmt.Errorf("ReceiveUpper: dropping packet with unknown MsgType 0x%x", b[0])
}

// TODO actually sign the thing
func (node *Node) SendPlaintext(route rovy.Route, codec uint64, b []byte) error {
	b = append(varint.ToUvarint(codec), b...) // XXX slowness

	pkt := &session.PlaintextPacket{
		Sender: node.peerid,
		Data:   b,
	}
	buf, err := pkt.MarshalBinary()
	if err != nil {
		return err
	}

	return node.forwarder.SendPacket(buf, node.PeerID(), route)
}

// TODO actually verify signature
func (node *Node) ReceiveUpperPlaintext(b []byte, route rovy.Route) error {
	pkt := &session.PlaintextPacket{}
	if err := pkt.UnmarshalBinary(b); err != nil {
		return err
	}

	codec, n, err := varint.FromUvarint(pkt.Data)
	if err != nil {
		return err
	}

	cb, present := node.handlers[codec]
	if !present {
		node.logger.Printf("ReceiveUpperPlaintext: dropping packet with unknown codec 0x%x", codec)
		return err
	}

	return cb(pkt.Data[n:], pkt.Sender, route)
}
