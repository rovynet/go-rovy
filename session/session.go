package session

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	multiaddr "github.com/multiformats/go-multiaddr"
	rovy "pkt.dev/go-rovy"
	ikpsk2 "pkt.dev/go-rovy/session/ikpsk2"
)

var (
	UnknownIndexError = errors.New("unknown receiver index on packet")
	SessionStateError = errors.New("prohibited session state transfer")
)

func init() {
	// TODO: do we need crypto/rand instead?
	rand.Seed(time.Now().UnixNano())
}

type Session struct {
	initiator    bool
	stage        int
	writer       func([]byte) error
	waiters      []chan error
	remoteAddr   multiaddr.Multiaddr
	remotePeerID rovy.PeerID
}

type SessionManager struct {
	privkey rovy.PrivateKey
	pubkey  rovy.PublicKey
	peerid  rovy.PeerID
	store   map[uint32]*Session
	logger  *log.Logger
}

func NewSessionManager(privkey rovy.PrivateKey, pubkey rovy.PublicKey, peerid rovy.PeerID, logger *log.Logger) *SessionManager {
	sm := &SessionManager{
		privkey: privkey,
		pubkey:  pubkey,
		peerid:  peerid,
		store:   make(map[uint32]*Session),
		logger:  logger,
	}
	return sm
}

func (sm *SessionManager) Insert(s *Session) uint32 {
	var idx uint32
	for {
		idx = rand.Uint32()
		_, present := sm.store[idx]
		if !present {
			break
		}
	}

	sm.store[idx] = s
	return idx
}

func (sm *SessionManager) Get(idx uint32) (s *Session, present bool) {
	s, present = sm.store[idx]
	return
}

func (sm *SessionManager) Find(peerid rovy.PeerID) (*Session, uint32, bool) {
	for idx, s := range sm.store {
		if s.remotePeerID == peerid {
			return s, idx, true
		}
	}
	return nil, 0, false
}

func (sm *SessionManager) Swap(idx1, idx2 uint32) {
	s, present := sm.store[idx1]
	if present {
		delete(sm.store, idx1)
	}

	// TODO: what if we overwrite an existing session here?
	sm.store[idx2] = s

	return
}

func (sm *SessionManager) Remove(idx uint32) {
	_, present := sm.store[idx]
	if present {
		delete(sm.store, idx)
	}
}

func (sm *SessionManager) CreateHello(peerid rovy.PeerID, raddr multiaddr.Multiaddr) *HelloPacket {
	s := &Session{
		initiator:    true,
		stage:        0x01,
		remoteAddr:   raddr,
		remotePeerID: peerid,
	}
	idx := sm.Insert(s)

	// _, msg := ikpsk2.WriteMessageA(s.handshake, []byte{})

	pkt := &HelloPacket{
		MsgType:     0x01,
		SenderIndex: idx,
		PeerID:      sm.peerid,
	}

	return pkt
}

func (sm *SessionManager) HandleHello(pkt *HelloPacket, raddr multiaddr.Multiaddr) *ResponsePacket {
	s := &Session{
		initiator:    false,
		stage:        0x02,
		remoteAddr:   raddr,
		remotePeerID: pkt.PeerID,
	}
	idx := sm.Insert(s)

	pkt2 := &ResponsePacket{
		MsgType:        0x02,
		ReceiverIndex:  idx,
		SenderIndex:    pkt.SenderIndex,
		ResponseHeader: ikpsk2.ResponseHeader{},
		// TODO: remove pkt.PeerID once we can extract peerid from handshake
		PeerID: sm.peerid,
	}

	return pkt2
}

func (sm *SessionManager) HandleHelloResponse(pkt *ResponsePacket, raddr multiaddr.Multiaddr) {
	s, present := sm.Get(pkt.SenderIndex)
	if !present || !s.initiator || s.stage != 0x01 {
		return
	}

	sm.Swap(pkt.SenderIndex, pkt.ReceiverIndex)

	s.stage = 0x03
	s.remoteAddr = raddr
	// TODO: remove pkt.PeerID once we can extract peerid from handshake
	// TODO: verify peerid against expected peerid from s.remotePeerID
	s.remotePeerID = pkt.PeerID
	for _, waiter := range s.waiters {
		waiter <- nil
	}
}

func (sm *SessionManager) CreateData(peerid rovy.PeerID, p []byte) (*DataPacket, multiaddr.Multiaddr, error) {
	s, idx, present := sm.Find(peerid)
	if !present {
		return nil, nil, fmt.Errorf("no session for %s", peerid)
	}

	pkt := &DataPacket{
		MsgType:       0x03,
		ReceiverIndex: idx,
		Data:          p,
	}
	return pkt, s.remoteAddr, nil
}

func (sm *SessionManager) HandleData(pkt *DataPacket, raddr multiaddr.Multiaddr) ([]byte, rovy.PeerID, error) {
	s, present := sm.store[pkt.ReceiverIndex]
	if !present {
		return nil, rovy.PeerID{}, UnknownIndexError
	}
	if !s.initiator && s.stage == 0x02 {
		s.stage = 0x03
		for _, waiter := range s.waiters {
			waiter <- nil
		}
	}
	if s.stage != 0x03 {
		return nil, rovy.PeerID{}, SessionStateError
	}

	return pkt.Data, s.remotePeerID, nil
}

func (sm *SessionManager) WaitFor(peerid rovy.PeerID) chan error {
	ch := make(chan error, 1)

	s, _, present := sm.Find(peerid)
	if !present {
		ch <- fmt.Errorf("no session for %s", peerid)
		return ch
	}

	s.waiters = append(s.waiters, ch)
	return ch
}
