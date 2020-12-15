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
	handshake    *ikpsk2.Handshake
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
	hs, err := ikpsk2.NewHandshakeInitiator(sm.privkey, peerid.PublicKey())
	if err != nil {
		return nil
	}
	hdr, _, err := hs.MakeHello(nil)
	if err != nil {
		return nil
	}

	s := &Session{
		initiator:    true,
		stage:        0x01,
		handshake:    hs,
		remoteAddr:   raddr,
		remotePeerID: peerid,
	}
	idx := sm.Insert(s)

	pkt := &HelloPacket{
		MsgType:     0x01,
		SenderIndex: idx,
		HelloHeader: hdr,
	}

	return pkt
}

func (sm *SessionManager) HandleHello(pkt *HelloPacket, raddr multiaddr.Multiaddr) *ResponsePacket {
	hs, err := ikpsk2.NewHandshakeResponder(sm.privkey)
	if err != nil {
		return nil
	}

	_, err = hs.ConsumeHello(pkt.HelloHeader, nil)
	if err != nil {
		return nil
	}

	hdr, _, err := hs.MakeResponse(nil)
	if err != nil {
		return nil
	}

	s := &Session{
		initiator:    false,
		stage:        0x02,
		handshake:    hs,
		remoteAddr:   raddr,
		remotePeerID: rovy.PeerID(hs.RemotePublicKey()),
	}
	idx := sm.Insert(s)

	pkt2 := &ResponsePacket{
		MsgType:        0x02,
		ReceiverIndex:  idx,
		SenderIndex:    pkt.SenderIndex,
		ResponseHeader: hdr,
	}

	return pkt2
}

func (sm *SessionManager) HandleHelloResponse(pkt *ResponsePacket, raddr multiaddr.Multiaddr) {
	s, present := sm.Get(pkt.SenderIndex)
	if !present || !s.initiator || s.stage != 0x01 {
		return
	}

	_, err := s.handshake.ConsumeResponse(pkt.ResponseHeader, nil)
	if err != nil {
		sm.logger.Printf("ConsumeResponse: %s", err)
		return
	}

	peerid := rovy.PeerID(s.handshake.RemotePublicKey())
	if peerid != s.remotePeerID {
		sm.Remove(pkt.SenderIndex)
		err = fmt.Errorf("expected PeerID %s, got %s", s.remoteAddr, peerid)
		for _, waiter := range s.waiters {
			waiter <- err
		}
		return
	}

	sm.Swap(pkt.SenderIndex, pkt.ReceiverIndex)

	s.stage = 0x03
	s.remoteAddr = raddr

	for _, waiter := range s.waiters {
		waiter <- nil
	}
}

func (sm *SessionManager) CreateData(peerid rovy.PeerID, p []byte) (*DataPacket, multiaddr.Multiaddr, error) {
	s, idx, present := sm.Find(peerid)
	if !present {
		return nil, nil, fmt.Errorf("no session for %s", peerid)
	}

	hdr, p2, err := s.handshake.MakeMessage(p)
	if err != nil {
		return nil, nil, err
	}

	pkt := &DataPacket{
		MsgType:       0x03,
		ReceiverIndex: idx,
		MessageHeader: hdr,
		Data:          p2,
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
		return nil, s.remotePeerID, SessionStateError
	}

	p2, err := s.handshake.ConsumeMessage(pkt.MessageHeader, pkt.Data)
	if err != nil {
		return nil, s.remotePeerID, err
	}

	return p2, s.remotePeerID, nil
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
