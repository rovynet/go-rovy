package session

import (
	"errors"
	"fmt"

	multiaddr "github.com/multiformats/go-multiaddr"
	rovy "github.com/rovynet/go-rovy"
	ikpsk2 "github.com/rovynet/go-rovy/session/ikpsk2"
)

var (
	UnknownIndexError = errors.New("unknown session index on packet")
	SessionStateError = errors.New("invalid session state transition")
)

const (
	HelloMsgType     = 0x1
	ResponseMsgType  = 0x2
	DataMsgType      = 0x4
	PlaintextMsgType = 0x5
)

type Session struct {
	initiator    bool
	stage        int
	waiters      []chan error
	handshake    *ikpsk2.Handshake
	remoteAddr   multiaddr.Multiaddr
	remotePeerID rovy.PeerID
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

func (s *Session) RemotePeerID() rovy.PeerID {
	return s.remotePeerID
}

func (s *Session) RemoteAddr() multiaddr.Multiaddr {
	return s.remoteAddr
}

func (s *Session) SetRemoteAddr(raddr multiaddr.Multiaddr) {
	s.remoteAddr = raddr
}

func (s *Session) CreateHello(pkt HelloPacket) (HelloPacket, error) {
	if !s.initiator {
		return pkt, SessionStateError
	}

	hdr, ct, err := s.handshake.MakeHello(pkt.Plaintext())
	if err != nil {
		return pkt, err
	}

	pkt.SetEphemeralKey(hdr.Ephemeral)
	pkt.SetStaticKey(hdr.Static)
	pkt.SetTimestamp(hdr.Timestamp)
	pkt = pkt.SetCiphertext(ct)

	return pkt, nil
}

func (s *Session) HandleHello(pkt HelloPacket) (HelloPacket, error) {
	if s.initiator {
		return pkt, SessionStateError
	}

	hdr := ikpsk2.HelloHeader{
		Ephemeral: pkt.EphemeralKey(),
		Static:    pkt.StaticKey(),
		Timestamp: pkt.Timestamp(),
	}

	ct := pkt.Ciphertext()
	pt, err := s.handshake.ConsumeHello(hdr, ct)
	if err != nil {
		return pkt, fmt.Errorf("HandleHello: %s: ct=%#v", err, ct)
	}

	pkt = pkt.SetPlaintext(pt)

	s.remotePeerID = rovy.NewPeerID(s.handshake.RemotePublicKey())

	return pkt, nil
}

func (s *Session) CreateResponse(pkt ResponsePacket) (ResponsePacket, error) {
	if s.initiator {
		return pkt, SessionStateError
	}

	hdr, ct, err := s.handshake.MakeResponse(pkt.Plaintext())
	if err != nil {
		return pkt, err
	}

	pkt.SetEphemeralKey(hdr.Ephemeral)
	pkt.SetEmpty(hdr.Empty)
	pkt = pkt.SetCiphertext(ct)

	return pkt, nil
}

func (s *Session) HandleHelloResponse(pkt ResponsePacket) (ResponsePacket, error) {
	if !s.initiator || s.stage != 0x01 {
		return pkt, SessionStateError
	}

	hdr := ikpsk2.ResponseHeader{
		Ephemeral: pkt.EphemeralKey(),
		Empty:     pkt.Empty(),
	}

	pt, err := s.handshake.ConsumeResponse(hdr, pkt.Ciphertext())
	if err != nil {
		return pkt, err
	}

	pkt = pkt.SetPlaintext(pt)

	peerid := rovy.NewPeerID(s.handshake.RemotePublicKey())
	if peerid != s.remotePeerID {
		err = fmt.Errorf("expected PeerID %s, got %s", s.remotePeerID, peerid)
		for _, waiter := range s.waiters {
			waiter <- err
		}
		return pkt, err
	}

	s.stage = 0x03

	for _, waiter := range s.waiters {
		waiter <- nil
	}

	return pkt, nil
}
