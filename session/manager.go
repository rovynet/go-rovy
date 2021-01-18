package session

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"

	multiaddr "github.com/multiformats/go-multiaddr"
	rovy "pkt.dev/go-rovy"
	ikpsk2 "pkt.dev/go-rovy/session/ikpsk2"
)

// TODO: make sure indexes from remote don't overwrite other sessions
type SessionManager struct {
	sync.RWMutex
	privkey       rovy.PrivateKey
	pubkey        rovy.PublicKey
	peerid        rovy.PeerID
	store         map[uint32]*Session
	logger        *log.Logger
	establishedCb EstablishedCb
}

type EstablishedCb func(rovy.PeerID)

func NewSessionManager(privkey rovy.PrivateKey, logger *log.Logger, cb EstablishedCb) *SessionManager {
	pubkey := privkey.PublicKey()
	sm := &SessionManager{
		privkey:       privkey,
		pubkey:        pubkey,
		peerid:        rovy.NewPeerID(pubkey),
		store:         make(map[uint32]*Session),
		logger:        logger,
		establishedCb: cb,
	}
	return sm
}

func (sm *SessionManager) randUint32() uint32 {
	var integer [4]byte
	for {
		if _, err := rand.Read(integer[:]); err != nil {
			sm.logger.Printf("can't read from crypto/rand: %s", err)
			time.Sleep(1 * time.Second)
		} else {
			return binary.LittleEndian.Uint32(integer[:])
		}
	}
}

func (sm *SessionManager) Insert(s *Session) uint32 {
	sm.Lock()
	defer sm.Unlock()

	var idx uint32
	for {
		idx = sm.randUint32()
		_, present := sm.store[idx]
		if !present {
			break
		}
	}

	sm.store[idx] = s
	return idx
}

func (sm *SessionManager) Get(idx uint32) (s *Session, present bool) {
	sm.RLock()
	defer sm.RUnlock()

	s, present = sm.store[idx]
	return
}

func (sm *SessionManager) Find(peerid rovy.PeerID) (*Session, uint32, bool) {
	sm.RLock()
	defer sm.RUnlock()

	for idx, s := range sm.store {
		if s.remotePeerID == peerid {
			return s, idx, true
		}
	}
	return nil, 0, false
}

func (sm *SessionManager) Swap(idx1, idx2 uint32) {
	sm.Lock()
	defer sm.Unlock()

	s, present := sm.store[idx1]
	if present {
		delete(sm.store, idx1)
	}

	sm.store[idx2] = s

	return
}

func (sm *SessionManager) Remove(idx uint32) {
	sm.Lock()
	defer sm.Unlock()

	_, present := sm.store[idx]
	if present {
		delete(sm.store, idx)
	}
}

func (sm *SessionManager) CreateHello(peerid rovy.PeerID, raddr multiaddr.Multiaddr) (*HelloPacket, error) {
	hs, err := ikpsk2.NewHandshakeInitiator(sm.privkey, peerid.PublicKey())
	if err != nil {
		return nil, err
	}

	s := newSession(peerid, hs)
	idx := sm.Insert(s)

	pkt, err := s.CreateHello(peerid, raddr)
	if err != nil {
		return nil, err
	}
	pkt.SenderIndex = idx

	// sm.logger.Printf("Hello: payload %#v", pkt.Payload)

	return pkt, nil
}

func (sm *SessionManager) HandleHello(pkt *HelloPacket, raddr multiaddr.Multiaddr) (*ResponsePacket, error) {
	hs, err := ikpsk2.NewHandshakeResponder(sm.privkey)
	if err != nil {
		return nil, err
	}

	// sm.logger.Printf("Hello: payload %#v", pkt.Payload)

	s := newSessionIncoming(hs)
	idx := sm.Insert(s)

	pkt2, err := s.HandleHello(pkt, raddr)
	if err != nil {
		return nil, err
	}
	pkt2.SenderIndex = pkt.SenderIndex
	pkt2.ReceiverIndex = idx

	// sm.logger.Printf("Response: payload %#v", pkt2.Payload)

	return pkt2, nil
}

func (sm *SessionManager) HandleHelloResponse(pkt *ResponsePacket, raddr multiaddr.Multiaddr) error {
	s, present := sm.Get(pkt.SenderIndex)
	if !present {
		return UnknownIndexError
	}

	// sm.logger.Printf("Response: payload %#v", pkt.Payload)

	err := s.HandleHelloResponse(pkt, raddr)
	if err != nil {
		// sm.Remove(idx)
		return err
	}

	sm.Swap(pkt.SenderIndex, pkt.ReceiverIndex)

	sm.establishedCb(s.remotePeerID)
	for _, waiter := range s.waiters {
		waiter <- nil
	}

	return nil
}

func (sm *SessionManager) CreateData(peerid rovy.PeerID, p []byte) (*DataPacket, multiaddr.Multiaddr, error) {
	s, idx, present := sm.Find(peerid)
	if !present {
		return nil, nil, fmt.Errorf("no session for %s", peerid)
	}

	pkt, raddr, err := s.CreateData(peerid, p)
	pkt.ReceiverIndex = idx

	return pkt, raddr, err
}

func (sm *SessionManager) HandleData(pkt *DataPacket, raddr multiaddr.Multiaddr) ([]byte, rovy.PeerID, error) {
	s, present := sm.Get(pkt.ReceiverIndex)
	if !present {
		return nil, rovy.PeerID{}, UnknownIndexError
	}

	stage := s.stage

	p, peerid, err := s.HandleData(pkt, raddr)
	if err != nil {
		return nil, rovy.NullPeerID, err
	}

	if stage == 0x02 {
		sm.establishedCb(s.remotePeerID)
		for _, waiter := range s.waiters {
			waiter <- nil
		}
	}

	return p, peerid, nil
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
