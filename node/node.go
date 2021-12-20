package node

import (
	"fmt"
	"log"
	"sync"

	pretty "github.com/kr/pretty"

	multiaddr "github.com/multiformats/go-multiaddr"
	multiaddrnet "github.com/multiformats/go-multiaddr/net"
	rovy "pkt.dev/go-rovy"
	forwarder "pkt.dev/go-rovy/forwarder"
	multigram "pkt.dev/go-rovy/multigram"
	routing "pkt.dev/go-rovy/routing"
	session "pkt.dev/go-rovy/session"
	ringbuf "pkt.dev/go-rovy/util/ringbuf"
)

const DirectUpperCodec = 0x12347

const LowerUnsealQueueSize = 1024
const LowerSealQueueSize = 1024
const HelloRecvQueueSize = 1024
const HelloSendQueueSize = 1024
const LowerMuxQueueSize = 1024

type Listener multiaddrnet.PacketConn

type UpperHandler func(rovy.UpperPacket) error
type LowerHandler func(rovy.LowerPacket) error

// TODO: move lower connection stuff to a Peering type (Connect, SendLower, Handle*)
type Node struct {
	peerid          rovy.PeerID
	logger          *log.Logger
	transports      []*Transport
	waiters         map[string][]chan error
	waitersLock     sync.Mutex
	sessions        *session.SessionManager
	upperHandlers   map[uint64]UpperHandler
	lowerHandlers   map[uint64]LowerHandler
	forwarder       *forwarder.Forwarder
	routing         *routing.Routing
	RxTpt           uint64
	RxLower         uint64
	RxUpper         uint64
	lowerUnsealQ    rovy.Queue
	lowerSealQ      rovy.Queue
	lowerHelloRecvQ rovy.Queue
	lowerHelloSendQ rovy.Queue
	lowerMuxQ       rovy.Queue
	upperHelloSendQ rovy.Queue
}

func NewNode(privkey rovy.PrivateKey, logger *log.Logger) *Node {
	pubkey := privkey.PublicKey()
	peerid := rovy.NewPeerID(pubkey)

	node := &Node{
		peerid:          peerid,
		logger:          logger,
		waiters:         map[string][]chan error{},
		upperHandlers:   map[uint64]UpperHandler{},
		lowerHandlers:   map[uint64]LowerHandler{},
		routing:         routing.NewRouting(logger),
		lowerUnsealQ:    ringbuf.NewRingBuffer(LowerUnsealQueueSize),
		lowerSealQ:      ringbuf.NewRingBuffer(LowerSealQueueSize),
		lowerHelloRecvQ: ringbuf.NewRingBuffer(HelloRecvQueueSize),
		lowerHelloSendQ: ringbuf.NewRingBuffer(HelloSendQueueSize),
		lowerMuxQ:       ringbuf.NewRingBuffer(LowerMuxQueueSize),
	}

	node.sessions = session.NewSessionManager(privkey, logger)

	node.forwarder = forwarder.NewForwarder(node.sessions.Multigram(), logger)
	node.forwarder.Attach(peerid, func(lpkt rovy.LowerPacket) error {
		upkt := rovy.NewUpperPacket(lpkt.Packet)
		return node.ReceiveUpper(upkt)
	})

	node.sessions.Multigram().AddCodec(DirectUpperCodec)

	go node.lowerUnsealRoutine()
	go node.lowerSealRoutine()
	go node.lowerHelloRecvRoutine()
	go node.lowerHelloSendRoutine()
	go node.lowerMuxRoutine()

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
	for _, lis := range node.transports {
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

func (node *Node) WaitFor(pid rovy.PeerID) error {
	node.waitersLock.Lock()

	spid := pid.String()

	_, present := node.waiters[spid]
	if !present {
		node.waiters[spid] = []chan error{}
	}

	ch := make(chan error, 1)
	node.waiters[spid] = append(node.waiters[spid], ch)
	node.waitersLock.Unlock()

	return <-ch
}

func (node *Node) connectedCallback(peerid rovy.PeerID, lower bool) {
	var err error

	if lower {
		slot, err := node.forwarder.Attach(peerid, func(lpkt rovy.LowerPacket) error {
			node.lowerSealQ.PutWithBackpressure(lpkt.Packet)
			return nil
		})
		if err != nil {
			err = fmt.Errorf("connected to %s, but forwarder error: %s", peerid, err)
		} else {
			node.routing.AddRoute(peerid, slot)
		}
	}

	if err == nil {
		node.Log().Printf("connected to %s", peerid)
	}

	node.waitersLock.Lock()
	defer node.waitersLock.Unlock()

	spid := peerid.String()

	w, present := node.waiters[spid]
	if present {
		for _, ch := range w {
			ch <- err
		}
		delete(node.waiters, spid)
	}
}

func (node *Node) Listen(lisaddr multiaddr.Multiaddr) error {
	tpt, err := NewTransport(lisaddr, node.logger)
	if err != nil {
		return err
	}

	node.transports = append(node.transports, tpt)

	go tpt.RecvRoutine(node.lowerUnsealQ)
	go tpt.SendRoutine()

	return nil
}

func (node *Node) Handle(codec uint64, cb UpperHandler) {
	_, present := node.upperHandlers[codec]
	if present {
		return
	}

	node.sessions.Multigram().AddCodec(codec)
	node.upperHandlers[codec] = cb
}

func (node *Node) HandleLower(codec uint64, cb LowerHandler) {
	_, present := node.lowerHandlers[codec]
	if present {
		return
	}

	node.sessions.Multigram().AddCodec(codec)
	node.lowerHandlers[codec] = cb
}

// TODO: timeouts
// TODO: check if we already have a session
func (node *Node) Connect(peerid rovy.PeerID, raddr multiaddr.Multiaddr) error {
	pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))

	if raddr != nil {
		pkt.LowerDst = peerid
		pkt.TptDst = raddr
		node.lowerHelloSendQ.Put(pkt)
	} else {
		// pkt.UpperDst = peerid
		// node.upperHelloSendQ.Put(pkt)

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

	if err := node.WaitFor(peerid); err != nil {
		node.Log().Printf("connect %s: %s", peerid, err)
		return err
	}

	return nil
}

func (node *Node) sendTransport(pkt rovy.Packet) error {
	node.transports[0].Send(pkt)
	return nil
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

	if upkt.RouteLen() == forwarder.HopLength {
		lpkt := rovy.NewLowerPacket(upkt.Packet)
		lpkt.SetCodec(node.SessionManager().Multigram().LookupNumber(DirectUpperCodec))
		lpkt.LowerSrc = node.PeerID()
		lpkt.LowerDst = upkt.UpperDst
		// return node.SendLower(lpkt)
		node.lowerSealQ.PutWithBackpressure(lpkt.Packet)
		return nil
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

	cb, present := node.upperHandlers[codec]
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
		resppkt, peerid, err := node.SessionManager().HandleResponse(resppkt, nil)
		if err != nil {
			return err
		}
		node.connectedCallback(peerid, false)
		return nil

	case session.DataMsgType:
		datapkt := session.NewDataPacket(upkt.Packet, rovy.UpperOffset, rovy.UpperPadding)

		peerid, firstdata, err := node.SessionManager().HandleData(datapkt)
		if err != nil {
			return err
		}

		if firstdata {
			node.connectedCallback(peerid, false)
		}

		datapkt.UpperSrc = peerid
		// node.logger.Printf("ReceiveUpper: datapkt=%#v", datapkt)
		node.Routing().AddRoute(datapkt.UpperSrc, upkt.Route().Reverse()) // XXX slowness

		upkt := rovy.NewUpperPacket(datapkt.Packet)
		return node.ReceiveUpperDirect(upkt)
	}

	return fmt.Errorf("ReceiveUpper: dropping packet with unknown MsgType 0x%x", msgtype)
}
