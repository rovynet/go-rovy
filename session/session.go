package session

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	multiaddr "github.com/multiformats/go-multiaddr"
	rovy "pkt.dev/go-rovy"
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

func (sm *SessionManager) CreateHello(peerid rovy.PeerID, raddr multiaddr.Multiaddr) *HelloPacket {
	s := &Session{
		initiator:    true,
		stage:        0x01,
		remoteAddr:   raddr,
		remotePeerID: peerid,
	}

	// Loop to make sure we don't overwrite an existing session, however unlikely.
	var idx uint32
	for {
		idx = rand.Uint32()
		_, present := sm.store[idx]
		if !present {
			sm.store[idx] = s
			break
		}
	}

	pkt := &HelloPacket{
		HelloHeader: HelloHeader{
			MsgType:     0x01,
			SenderIndex: idx,
		},
		ObservedAddr: raddr,
		ObservedMTU:  rovy.PreliminaryMTU,
		PeerID:       sm.peerid,
	}

	return pkt
}

func (sm *SessionManager) HandleHello(pkt *HelloPacket, raddr multiaddr.Multiaddr) *HelloResponsePacket {
	s := &Session{
		initiator:    false,
		stage:        0x02,
		remoteAddr:   raddr,
		remotePeerID: pkt.PeerID,
	}

	// Make sure we don't overwrite an existing session, however unlikely.
	var idx uint32
	for {
		idx = rand.Uint32()
		_, present := sm.store[idx]
		if !present {
			sm.store[idx] = s
			break
		}
	}

	pkt2 := &HelloResponsePacket{
		HelloResponseHeader: HelloResponseHeader{
			MsgType:       0x02,
			ReceiverIndex: idx,
			SenderIndex:   pkt.SenderIndex,
		},
		ObservedMTU:  rovy.PreliminaryMTU,
		ObservedAddr: raddr,
		PeerID:       sm.peerid,
	}

	return pkt2
}

func (sm *SessionManager) HandleHelloResponse(pkt *HelloResponsePacket, raddr multiaddr.Multiaddr) {
	s, present := sm.store[pkt.SenderIndex]
	if !present || !s.initiator || s.stage != 0x01 {
		return
	}

	delete(sm.store, pkt.SenderIndex)
	// TODO: make sure we don't overwrite an existing session
	sm.store[pkt.ReceiverIndex] = s

	s.stage = 0x03
	s.remoteAddr = raddr
	s.remotePeerID = pkt.PeerID
	for _, waiter := range s.waiters {
		waiter <- nil
	}
}

func (sm *SessionManager) CreateData(peerid rovy.PeerID, p []byte) (*DataPacket, multiaddr.Multiaddr, error) {
	for idx, s := range sm.store {
		if s.remotePeerID == peerid {
			pkt := &DataPacket{
				DataHeader: DataHeader{
					MsgType:       0x03,
					ReceiverIndex: idx,
				},
				Data: p,
			}
			return pkt, s.remoteAddr, nil
		}
	}
	return nil, nil, fmt.Errorf("no session for %s", peerid)
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
	for _, s := range sm.store {
		if s.remotePeerID == peerid {
			s.waiters = append(s.waiters, ch)
			return ch
		}
	}
	ch <- fmt.Errorf("no session for %s", peerid)
	return ch
}
