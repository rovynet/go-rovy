package node

import (
	"errors"
	"fmt"
	"log"
	"net/netip"
	"sync"

	rovy "go.rovy.net"
	rapi "go.rovy.net/api"
	forwarder "go.rovy.net/node/forwarder"
	routing "go.rovy.net/node/routing"
	service "go.rovy.net/node/service"
	session "go.rovy.net/node/session"
	ringbuf "go.rovy.net/node/util/ringbuf"
)

const DirectUpperCodec = 0x12347

const DefaultQueueSize = 1024

var ErrRunning = errors.New("routines are already running")
var ErrNotRunning = errors.New("routines are not running")

type UpperHandler func(rovy.UpperPacket) error
type LowerHandler func(rovy.LowerPacket) error

// TODO: move lower connection stuff to a Peering type (Connect, SendLower, Handle*)
type Node struct {
	peerid        rovy.PeerID
	logger        *log.Logger
	transports    []*Transport
	waiters       map[rovy.PeerID][]chan error
	waitersLock   sync.Mutex
	sessions      *session.SessionManager
	upperHandlers map[uint64]UpperHandler
	lowerHandlers map[uint64]LowerHandler
	forwarder     *forwarder.Forwarder
	routing       *routing.Routing
	services      *service.ServiceManager

	running    chan int
	helloSendQ *ringbuf.RingBuffer
	lowerSendQ *ringbuf.RingBuffer
	upperSendQ *ringbuf.RingBuffer
	helloRecvQ *ringbuf.RingBuffer
	lowerRecvQ *ringbuf.RingBuffer
	lowerMuxQ  *ringbuf.RingBuffer
	upperRecvQ *ringbuf.RingBuffer
	upperMuxQ  *ringbuf.RingBuffer
}

func NewNode(privkey rovy.PrivateKey, logger *log.Logger) *Node {
	pubkey := privkey.PublicKey()
	peerid := rovy.NewPeerID(pubkey)

	node := &Node{
		peerid:        peerid,
		logger:        logger,
		waiters:       map[rovy.PeerID][]chan error{},
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
	node.services = service.NewServiceManager(logger)

	node.forwarder = forwarder.NewForwarder(logger)
	node.forwarder.Attach(peerid, func(lpkt rovy.LowerPacket) error {
		node.upperRecvQ.Put(lpkt.Packet)
		return nil
	})

	return node
}

func (node *Node) Start() (rapi.NodeInfo, error) {
	var ni rapi.NodeInfo

	if node.Running() {
		return ni, ErrRunning
	}

	node.running = make(chan int)
	go node.helloSendRoutine()
	go node.lowerSendRoutine()
	go node.upperSendRoutine()
	go node.helloRecvRoutine()
	go node.lowerRecvRoutine()
	go node.lowerMuxRoutine()
	go node.upperRecvRoutine()
	go node.upperMuxRoutine()

	for _, tpt := range node.transports {
		tpt.Start(node.lowerRecvQ)
	}

	ni, _ = node.Info()
	return ni, nil
}

func (node *Node) Stop() (rapi.NodeInfo, error) {
	var ni rapi.NodeInfo

	if !node.Running() {
		return ni, ErrNotRunning
	}

	close(node.running)

	for _, tpt := range node.transports {
		tpt.Stop()
	}

	ni, _ = node.Info()
	return ni, nil
}

func (node *Node) Running() bool {
	if node.running != nil {
		select {
		case <-node.running:
			// we're not running anymore, channel is closed.
			// the channel is unbuffered, so if we never write anything to it,
			// then the only way to reach here is if the channel is closed.
			return false
		default:
			return true
		}
	}
	return false
}

func (node *Node) PeerID() rovy.PeerID {
	return node.peerid
}

func (node *Node) IPAddr() netip.Addr {
	return node.peerid.PublicKey().IPAddr()
}

func (node *Node) Log() *log.Logger {
	return node.logger
}

func (node *Node) Addresses() (addrs []rovy.Multiaddr) {
	for _, lis := range node.transports {
		ma := lis.LocalMultiaddr()
		ma.PeerID = node.PeerID()
		addrs = append(addrs, ma)
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

func (node *Node) Services() *service.ServiceManager {
	return node.services
}

func (node *Node) WaitFor(pid rovy.PeerID) error {
	node.waitersLock.Lock()

	_, present := node.waiters[pid]
	if !present {
		node.waiters[pid] = []chan error{}
	}

	ch := make(chan error, 1)
	node.waiters[pid] = append(node.waiters[pid], ch)
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

	w, present := node.waiters[peerid]
	if present {
		for _, ch := range w {
			ch <- err
		}
		delete(node.waiters, peerid)
	}
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

// TODO: pick correct transport
func (node *Node) sendTransport(pkt rovy.Packet) error {
	return node.transports[0].Send(pkt)
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
