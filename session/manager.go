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
	multigram "pkt.dev/go-rovy/multigram"
	ikpsk2 "pkt.dev/go-rovy/session/ikpsk2"
)

// TODO: make sure indexes from remote don't overwrite other sessions
type SessionManager struct {
	sync.RWMutex
	privkey       rovy.PrivateKey
	pubkey        rovy.PublicKey
	peerid        rovy.PeerID
	store         map[uint32]*Session
	multigram     *multigram.Table
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
		multigram:     multigram.NewTable(),
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

func (sm *SessionManager) Multigram() *multigram.Table {
	return sm.multigram
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

func (sm *SessionManager) CreateHello(pkt HelloPacket, peerid rovy.PeerID, raddr multiaddr.Multiaddr) (HelloPacket, error) {
	hs, err := ikpsk2.NewHandshakeInitiator(sm.privkey, peerid.PublicKey())
	if err != nil {
		return pkt, err
	}

	s := newSession(peerid, hs)
	idx := sm.Insert(s)

	pkt.SetSenderIndex(idx)
	pkt = pkt.SetPlaintext(sm.multigram.Bytes())

	if raddr != nil {
		s.SetRemoteAddr(raddr)
	}
	return s.CreateHello(pkt)
}

func (sm *SessionManager) HandleHello(pkt HelloPacket, raddr multiaddr.Multiaddr) (ResponsePacket, error) {
	var pkt2 ResponsePacket

	hs, err := ikpsk2.NewHandshakeResponder(sm.privkey)
	if err != nil {
		return pkt2, err
	}

	s := newSessionIncoming(hs)

	pkt, err = s.HandleHello(pkt)
	if err != nil {
		return pkt2, fmt.Errorf("HandleHello: %s", err)
	}

	mgram, err := multigram.NewTableFromBytes(pkt.Plaintext())
	if err != nil {
		return pkt2, err
	}

	pkt2 = NewResponsePacket(rovy.NewPacket(make([]byte, rovy.TptMTU)), pkt.Offset, pkt.Padding)
	pkt2.SetSenderIndex(pkt.SenderIndex())
	pkt2 = pkt2.SetPlaintext(sm.multigram.Bytes())

	pkt2, err = s.CreateResponse(pkt2)
	if err != nil {
		return pkt2, err
	}
	pkt2.SetSessionIndex(sm.Insert(s))

	s.remotePeerID = rovy.NewPeerID(s.handshake.RemotePublicKey())
	s.SetRemoteMultigram(mgram)

	if raddr != nil {
		s.SetRemoteAddr(raddr)
	}

	return pkt2, nil
}

func (sm *SessionManager) HandleResponse(pkt ResponsePacket, raddr multiaddr.Multiaddr) (ResponsePacket, error) {
	s, present := sm.Get(pkt.SenderIndex())
	if !present {
		return pkt, UnknownIndexError
	}

	pkt, err := s.HandleHelloResponse(pkt)
	if err != nil {
		return pkt, err
	}

	mgram, err := multigram.NewTableFromBytes(pkt.Plaintext())
	if err != nil {
		return pkt, err
	}

	sm.Swap(pkt.SenderIndex(), pkt.SessionIndex())

	s.SetRemoteMultigram(mgram)

	if raddr != nil {
		s.SetRemoteAddr(raddr)
	}

	// With raddr = nil we signal that this session is only via the forwarder
	// and mustn't have the ConnectedLower callback called.
	if raddr != nil {
		sm.establishedCb(s.remotePeerID)
	}

	for _, waiter := range s.waiters {
		waiter <- nil
	}

	return pkt, nil
}

func (sm *SessionManager) CreateData(pkt DataPacket, peerid rovy.PeerID) (multiaddr.Multiaddr, error) {
	s, idx, present := sm.Find(peerid)
	if !present {
		return nil, fmt.Errorf("no session for %s", peerid)
	}

	hdr, ct, err := s.handshake.MakeMessage(pkt.Plaintext())
	if err != nil {
		return nil, err
	}

	pkt.SetMsgType(DataMsgType)
	pkt.SetSessionIndex(idx)
	pkt.SetNonce(hdr.Nonce)
	pkt = pkt.SetCiphertext(ct)

	return s.remoteAddr, nil
}

func (sm *SessionManager) HandleData(pkt DataPacket) (rovy.PeerID, error) {
	s, present := sm.Get(pkt.SessionIndex())
	if !present {
		return rovy.EmptyPeerID, UnknownIndexError
	}
	stage := s.stage

	hdr := ikpsk2.MessageHeader{Nonce: pkt.Nonce()}
	payloadPlain, err := s.handshake.ConsumeMessage(hdr, pkt.Ciphertext())
	if err != nil {
		return rovy.EmptyPeerID, err
	}

	// TODO: instead aead.Open should reuse storage
	// XXX: why are we discarding the returned Packet?
	pkt = pkt.SetPlaintext(payloadPlain)

	if stage == 0x03 {
		return s.remotePeerID, nil
	}
	s.stage = 0x03

	// We only call the ConnectedLower callback for lower packets.
	// We're dealing with a lower packet if LowerSrc isn't set yet.
	if pkt.LowerSrc.Empty() {
		// sm.logger.Printf("calling establishedCb now for %s", s.RemotePeerID())
		sm.establishedCb(s.remotePeerID)
		for _, waiter := range s.waiters {
			waiter <- nil
		}
	}

	return s.remotePeerID, nil
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
