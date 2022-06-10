package node

import (
	"fmt"
	"log"
	"net/netip"
	"sync"

	multiaddrnet "github.com/multiformats/go-multiaddr/net"
	rovy "go.rovy.net"
	forwarder "go.rovy.net/forwarder"
	routing "go.rovy.net/routing"
	session "go.rovy.net/session"
	ringbuf "go.rovy.net/util/ringbuf"
)

const DirectUpperCodec = 0x12347

const DefaultQueueSize = 1024

type Listener multiaddrnet.PacketConn

type UpperHandler func(rovy.UpperPacket) error
type LowerHandler func(rovy.LowerPacket) error

// TODO: move lower connection stuff to a Peering type (Connect, SendLower, Handle*)
type Node struct {
	peerid        rovy.PeerID
	logger        *log.Logger
	transports    []*Transport
	waiters       map[string][]chan error
	waitersLock   sync.Mutex
	sessions      *session.SessionManager
	upperHandlers map[uint64]UpperHandler
	lowerHandlers map[uint64]LowerHandler
	forwarder     *forwarder.Forwarder
	routing       *routing.Routing

	RxTpt   uint64
	RxLower uint64
	RxUpper uint64

	helloSendQ rovy.Queue
	lowerSendQ rovy.Queue
	upperSendQ rovy.Queue

	helloRecvQ rovy.Queue
	lowerRecvQ rovy.Queue
	lowerMuxQ  rovy.Queue
	upperRecvQ rovy.Queue
	upperMuxQ  rovy.Queue
}

func NewNode(privkey rovy.PrivateKey, logger *log.Logger) *Node {
	pubkey := privkey.PublicKey()
	peerid := rovy.NewPeerID(pubkey)

	node := &Node{
		peerid:        peerid,
		logger:        logger,
		waiters:       map[string][]chan error{},
		upperHandlers: map[uint64]UpperHandler{},
		lowerHandlers: map[uint64]LowerHandler{},
		routing:       routing.NewRouting(logger),
		helloSendQ:    ringbuf.NewRingBuffer(DefaultQueueSize),
		lowerSendQ:    ringbuf.NewRingBuffer(DefaultQueueSize),
		upperSendQ:    ringbuf.NewRingBuffer(DefaultQueueSize),
		helloRecvQ:    ringbuf.NewRingBuffer(DefaultQueueSize),
		lowerRecvQ:    ringbuf.NewRingBuffer(DefaultQueueSize),
		lowerMuxQ:     ringbuf.NewRingBuffer(DefaultQueueSize),
		upperRecvQ:    ringbuf.NewRingBuffer(DefaultQueueSize),
		upperMuxQ:     ringbuf.NewRingBuffer(DefaultQueueSize),
	}

	node.sessions = session.NewSessionManager(privkey, logger)

	node.forwarder = forwarder.NewForwarder(logger)
	node.forwarder.Attach(peerid, func(lpkt rovy.LowerPacket) error {
		node.upperRecvQ.Put(lpkt.Packet)
		return nil
	})

	go node.helloSendRoutine()
	go node.lowerSendRoutine()
	go node.upperSendRoutine()

	go node.helloRecvRoutine()
	go node.lowerRecvRoutine()
	go node.lowerMuxRoutine()
	go node.upperRecvRoutine()
	go node.upperMuxRoutine()

	return node
}

func (node *Node) PeerID() rovy.PeerID {
	return node.peerid
}

func (node *Node) Multiaddr() rovy.Multiaddr {
	return rovy.Multiaddr{PeerID: node.peerid}
}

func (node *Node) IPAddr() netip.Addr {
	return node.peerid.PublicKey().IPAddr()
}

func (node *Node) Log() *log.Logger {
	return node.logger
}

func (node *Node) Adresses() (addrs []rovy.Multiaddr) {
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
			node.lowerSendQ.PutWithBackpressure(lpkt.Packet)
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

func (node *Node) Listen(lisaddr rovy.Multiaddr) error {
	tpt, err := NewTransport(lisaddr, node.logger)
	if err != nil {
		return err
	}

	node.transports = append(node.transports, tpt)

	go tpt.RecvRoutine(node.lowerRecvQ)
	go tpt.SendRoutine()

	return nil
}

func (node *Node) Handle(codec uint64, cb UpperHandler) {
	_, present := node.upperHandlers[codec]
	if present {
		return
	}

	node.upperHandlers[codec] = cb
}

func (node *Node) HandleLower(codec uint64, cb LowerHandler) {
	_, present := node.lowerHandlers[codec]
	if present {
		return
	}

	node.lowerHandlers[codec] = cb
}

// TODO: timeouts
// TODO: check if we already have a session
func (node *Node) Connect(peerid rovy.PeerID, raddr rovy.Multiaddr) error {
	pkt := rovy.NewPacket(make([]byte, rovy.TptMTU))

	if !raddr.Empty() {
		pkt.LowerDst = peerid
		pkt.TptDst = raddr
		node.helloSendQ.Put(pkt)
	} else {
		pkt.UpperDst = peerid
		node.helloSendQ.Put(pkt)
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
	upkt.SetCodec(codec)
	upkt.SetRoute(route)
	upkt = upkt.SetPayload(p)

	return node.SendUpper(upkt)
}

func (node *Node) SendUpper(upkt rovy.UpperPacket) error {
	node.upperSendQ.PutWithBackpressure(upkt.Packet)
	return nil
}
