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

type DataHandler func([]byte, rovy.PeerID) error

// TODO: move lower connection stuff to a Peering type (Connect, SendLower, Handle*)
type Node struct {
	pubkey    rovy.PublicKey // XXX unused
	peerid    rovy.PeerID
	logger    *log.Logger
	listeners []Listener
	sessions  *session.SessionManager
	handlers  map[uint64]DataHandler
	forwarder *forwarder.Forwarder
	routing   *routing.Routing
}

func NewNode(privkey rovy.PrivateKey, logger *log.Logger) *Node {
	pubkey := privkey.PublicKey()
	peerid := rovy.NewPeerID(pubkey)

	node := &Node{
		pubkey:   pubkey,
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

		route := rovy.Route(p[2+clen : 2+llen+clen]).Reverse()
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

			maddr, _ := multiaddrnet.FromNetAddr(raddr)
			if err = node.ReceiveLower(p[:n], maddr); err != nil {
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
		// node.logger.Printf("UnmarshalBinary: %s", err)
		return nil, rovy.NullPeerID, err
	}

	data, peerid, err := node.sessions.HandleData(pkt, maddr)
	if err != nil {
		// node.logger.Printf("handleDataPacket: %s: %s", maddr, err)
		return nil, rovy.NullPeerID, err
	}

	return data, peerid, nil
}

func (node *Node) handleHelloPacket(p []byte, maddr multiaddr.Multiaddr) ([]byte, error) {
	pkt := &session.HelloPacket{}
	if err := pkt.UnmarshalBinary(p); err != nil {
		// node.logger.Printf("UnmarshalBinary: %s", err)
		return nil, err
	}

	pkt2, err := node.sessions.HandleHello(pkt, maddr)
	if err != nil {
		// node.logger.Printf("HandleHello: %s", err)
		return nil, err
	}

	buf, err := pkt2.MarshalBinary()
	if err != nil {
		// node.logger.Printf("MarshalBinary: %s", err)
		return nil, err
	}

	return buf, nil
}

func (node *Node) handleResponsePacket(p []byte, maddr multiaddr.Multiaddr) error {
	pkt := &session.ResponsePacket{}

	if err := pkt.UnmarshalBinary(p); err != nil {
		// node.logger.Printf("UnmarshalBinary: %s", err)
		return err
	}

	if err := node.sessions.HandleHelloResponse(pkt, maddr); err != nil {
		// node.logger.Printf("HandleHelloResponse: %s", err)
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
		if _, err = node.listeners[0].WriteToMultiaddr(buf, raddr); err != nil {
			node.logger.Printf("Connect: WriteTo: %s", err)
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
	pkt, raddr, err := node.sessions.CreateData(pid, p)
	if err != nil {
		return err
	}

	// node.logger.Printf("SendLower: pkt=%#v", pkt)

	buf, err := pkt.MarshalBinary()
	if err != nil {
		return err
	}
	// node.logger.Printf("SendLower: sending: %s len=%d %#v", raddr, len(buf), buf)
	_, err = node.listeners[0].WriteToMultiaddr(buf, raddr)
	return err
}

func (node *Node) ReceiveLower(p []byte, maddr multiaddr.Multiaddr) error {
	// node.logger.Printf("ReceiveLower: got from %s: %#v", maddr, p)

	switch p[0] {
	case 0x01:
		buf, err := node.handleHelloPacket(p, maddr)
		if err != nil {
			return err
		}
		if _, err = node.listeners[0].WriteToMultiaddr(buf, maddr); err != nil {
			return err
		}
	case 0x02:
		if err := node.handleResponsePacket(p, maddr); err != nil {
			return err
		}
	case 0x04:
		data, peerid, err := node.handleDataPacket(p, maddr)
		if err != nil {
			return err
		}

		codec, cn, err := node.sessions.Multigram().FromUvarint(data)
		if err != nil {
			// node.logger.Printf("ReceiveLower: multigram: %s", err)
			return err
		}

		switch codec {
		case forwarder.DataMulticodec:
			return node.forwarder.HandlePacket(data, cn, peerid)
		case forwarder.ErrorMulticodec:
			return node.forwarder.HandleError(data, peerid)
		default:
			return node.ReceiveUpperDirect(peerid, data, maddr)
		}
	default:
		node.logger.Printf("ReceiveLower: dropping packet with unknown MsgType 0x%x", p[0])
	}

	return nil
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

	if len(route) == forwarder.HopLength {
		// node.logger.Printf("SendUpper: sending to direct peer")
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

func (node *Node) ReceiveUpperDirect(from rovy.PeerID, data []byte, maddr multiaddr.Multiaddr) error {
	sess, _, present := node.SessionManager().Find(from)
	if !present {
		node.logger.Printf("lost track of session while handling packet from %s", from)
		return nil // XXX return error instead?
	}

	codec, n, err := sess.Multigram().FromUvarint(data)
	if err != nil {
		// node.logger.Printf("ReceiveUpperDirect: multigram: %s", err)
		return err
	}

	// node.logger.Printf("ReceiveUpperDirect: codec=0x%x n=%d data=%#v", codec, n, data[:])
	cb, present := node.handlers[codec]
	if !present {
		node.logger.Printf("ReceiveUpperDirect: dropping packet with unknown codec %d", codec)
		return err
	}

	// node.logger.Printf("ReceiveUpper: cb=%#v", cb)
	return cb(data[n:], from)
}

// TODO need to actually send helloResponse packet
func (node *Node) ReceiveUpper(from rovy.PeerID, b []byte, route rovy.Route) error {
	// node.logger.Printf("ReceiveUpper: got from /rovyrt/%s: %#v", route, b)

	switch b[0] {
	case 0x01:
		buf, err := node.handleHelloPacket(b, nil)
		if err != nil {
			return err
		}

		if err = node.forwarder.SendPacket(buf, from, route); err != nil {
			node.logger.Printf("ReceiveUpper: SendPacket: %s", err)
			return err
		}
	case 0x02:
		if err := node.handleResponsePacket(b, nil); err != nil {
			return err
		}
	case 0x04:
		data, peerid, err := node.handleDataPacket(b, nil)
		if err != nil {
			return err
		}

		return node.ReceiveUpperDirect(peerid, data, nil)
	default:
		node.logger.Printf("ReceiveUpper: dropping packet with unknown MsgType 0x%x", b[0])
	}

	return nil
}
