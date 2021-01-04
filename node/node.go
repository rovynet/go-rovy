package node

import (
	"log"
	"net"

	multiaddr "github.com/multiformats/go-multiaddr"
	multiaddrnet "github.com/multiformats/go-multiaddr/net"
	rovy "pkt.dev/go-rovy"
	session "pkt.dev/go-rovy/session"
)

type Listener multiaddrnet.PacketConn

type DataHandler func([]byte, rovy.PeerID)

type Node struct {
	pubkey    rovy.PublicKey
	peerid    rovy.PeerID
	logger    *log.Logger
	listeners []Listener
	sessions  *session.SessionManager
	handler   DataHandler
}

func NewNode(privkey rovy.PrivateKey, logger *log.Logger) *Node {
	pubkey := privkey.PublicKey()
	node := &Node{
		pubkey:   pubkey,
		peerid:   rovy.NewPeerID(pubkey),
		logger:   logger,
		sessions: session.NewSessionManager(privkey, logger),
	}
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
			node.handlePacket(p[:], n, raddr)
		}
	}()

	return nil
}

func (node *Node) Handle(cb DataHandler) {
	node.handler = cb
}

func (node *Node) handlePacket(p []byte, n int, raddr net.Addr) {
	// node.logger.Printf("got: %+v", p)
	switch p[0] {
	case 0x01:
		node.handleHelloPacket(p, n, raddr)
	case 0x02:
		node.handleResponsePacket(p, n, raddr)
	case 0x03:
		node.handleDataPacket(p, n, raddr)
	}
}

func (node *Node) handleDataPacket(p []byte, n int, raddr net.Addr) {
	pkt := &session.DataPacket{}
	if err := pkt.UnmarshalBinary(p[:n]); err != nil {
		node.logger.Printf("UnmarshalBinary: %s %+v", err, p[:n])
		return
	}

	maddr, _ := multiaddrnet.FromNetAddr(raddr)
	data, peerid, err := node.sessions.HandleData(pkt, maddr)
	if err != nil {
		node.logger.Printf("handleDataPacket: %s: %s", maddr, err)
		return
	}

	if node.handler != nil {
		node.handler(data, peerid)
	}
	return
}

func (node *Node) handleHelloPacket(p []byte, n int, raddr net.Addr) {
	pkt := &session.HelloPacket{}
	if err := pkt.UnmarshalBinary(p[:n]); err != nil {
		node.logger.Printf("UnmarshalBinary: %s", err)
		return
	}

	maddr, _ := multiaddrnet.FromNetAddr(raddr)
	pkt2, err := node.sessions.HandleHello(pkt, maddr)
	if err != nil {
		node.logger.Printf("HandleHello: %s", err)
		return
	}

	buf, err := pkt2.MarshalBinary()
	if err != nil {
		node.logger.Printf("MarshalBinary: %s", err)
		return
	}

	if _, err = node.listeners[0].WriteToMultiaddr(buf, maddr); err != nil {
		node.logger.Printf("WriteTo: %s", err)
		return
	}
	return
}

func (node *Node) handleResponsePacket(p []byte, n int, raddr net.Addr) {
	pkt := &session.ResponsePacket{}

	if err := pkt.UnmarshalBinary(p[:n]); err != nil {
		node.logger.Printf("UnmarshalBinary: %s", err)
		return
	}

	maddr, _ := multiaddrnet.FromNetAddr(raddr)
	if err := node.sessions.HandleHelloResponse(pkt, maddr); err != nil {
		node.logger.Printf("HandleHelloResponse: %s", err)
		return
	}
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

	if _, err = node.listeners[0].WriteToMultiaddr(buf, raddr); err != nil {
		node.logger.Printf("WriteTo: %s", err)
		return err
	}

	if err = <-node.sessions.WaitFor(peerid); err != nil {
		node.Log().Printf("connect %s: %s", peerid, err)
	}
	return err
}

func (node *Node) Send(pid rovy.PeerID, p []byte) error {
	pkt, raddr, err := node.sessions.CreateData(pid, p)
	if err != nil {
		return err
	}

	buf, err := pkt.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = node.listeners[0].WriteToMultiaddr(buf, raddr)
	return err
}
