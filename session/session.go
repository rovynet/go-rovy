package session

import (
	"errors"
	"fmt"

	multiaddr "github.com/multiformats/go-multiaddr"
	rovy "pkt.dev/go-rovy"
	multigram "pkt.dev/go-rovy/multigram"
	ikpsk2 "pkt.dev/go-rovy/session/ikpsk2"
)

var (
	UnknownIndexError = errors.New("unknown session index on packet")
	SessionStateError = errors.New("invalid session state transition")

	StubHandshakePayload = []byte{0xa, 0xc, 0xa, 0xb}
)

type Session struct {
	initiator       bool
	stage           int
	writer          func([]byte) error // XXX unused?
	waiters         []chan error
	handshake       *ikpsk2.Handshake
	remoteAddr      multiaddr.Multiaddr // XXX unused?
	remotePeerID    rovy.PeerID
	remoteMultigram multigram.Table
}

func newSession(peerid rovy.PeerID, hs *ikpsk2.Handshake) *Session {
	return &Session{
		initiator:    true,
		stage:        0x01,
		handshake:    hs,
		remotePeerID: peerid,
	}
}

func newSessionIncoming(hs *ikpsk2.Handshake) *Session {
	return &Session{
		initiator: false,
		stage:     0x02,
		handshake: hs,
	}
}

func (s *Session) CreateHello(peerid rovy.PeerID, raddr multiaddr.Multiaddr, mgram multigram.Table) (*HelloPacket, error) {
	if !s.initiator {
		return nil, SessionStateError
	}

	hdr, payload, err := s.handshake.MakeHello(mgram.Bytes())
	if err != nil {
		return nil, err
	}

	s.remoteAddr = raddr
	s.remotePeerID = peerid

	return &HelloPacket{
		MsgType:     0x01,
		HelloHeader: hdr,
		Payload:     payload,
	}, nil
}

func (s *Session) HandleHello(pkt *HelloPacket, raddr multiaddr.Multiaddr, mgram multigram.Table) (*ResponsePacket, error) {
	if s.initiator {
		return nil, SessionStateError
	}

	payload, err := s.handshake.ConsumeHello(pkt.HelloHeader, pkt.Payload)
	if err != nil {
		return nil, err
	}

	// if !bytes.Equal(payload, StubHandshakePayload) {
	// 	return nil, fmt.Errorf("expected handshake payload %#v, got %#v", StubHandshakePayload, payload)
	// }

	remoteMgram, err := multigram.NewTableFromBytes(payload)
	if err != nil {
		return nil, err
	}

	hdr, payload2, err := s.handshake.MakeResponse(mgram.Bytes())
	if err != nil {
		return nil, err
	}

	s.remoteAddr = raddr
	s.remotePeerID = rovy.PeerID(s.handshake.RemotePublicKey())
	s.remoteMultigram = remoteMgram

	return &ResponsePacket{
		MsgType:        0x02,
		ResponseHeader: hdr,
		Payload:        payload2,
	}, nil
}

func (s *Session) HandleHelloResponse(pkt *ResponsePacket, raddr multiaddr.Multiaddr) error {
	if !s.initiator || s.stage != 0x01 {
		return SessionStateError
	}

	payload, err := s.handshake.ConsumeResponse(pkt.ResponseHeader, pkt.Payload)
	if err != nil {
		return err
	}

	// if !bytes.Equal(payload, StubHandshakePayload) {
	// 	return fmt.Errorf("expected handshake payload %#v, got %#v", StubHandshakePayload, payload)
	// }

	mgram, err := multigram.NewTableFromBytes(payload)
	if err != nil {
		return err
	}

	peerid := rovy.PeerID(s.handshake.RemotePublicKey())
	if peerid != s.remotePeerID {
		err = fmt.Errorf("expected PeerID %s, got %s", s.remotePeerID, peerid)
		for _, waiter := range s.waiters {
			waiter <- err
		}
		return err
	}

	s.stage = 0x03
	s.remoteAddr = raddr
	s.remoteMultigram = mgram

	for _, waiter := range s.waiters {
		waiter <- nil
	}

	return nil
}

func (s *Session) CreateData(peerid rovy.PeerID, p []byte) (*DataPacket, multiaddr.Multiaddr, error) {
	hdr, p2, err := s.handshake.MakeMessage(p)
	if err != nil {
		return nil, nil, err
	}

	pkt := &DataPacket{
		MsgType:       0x04,
		MessageHeader: hdr,
		Data:          p2,
	}
	return pkt, s.remoteAddr, nil
}

func (s *Session) HandleData(pkt *DataPacket, raddr multiaddr.Multiaddr) ([]byte, rovy.PeerID, error) {
	p2, err := s.handshake.ConsumeMessage(pkt.MessageHeader, pkt.Data)
	if err != nil {
		return nil, s.remotePeerID, err
	}

	s.stage = 0x03

	return p2, s.remotePeerID, nil
}
