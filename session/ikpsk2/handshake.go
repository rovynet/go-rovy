package ikpsk2

import (
	"fmt"
	"log"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/poly1305"
	"golang.zx2c4.com/wireguard/tai64n"

	rovy "pkt.dev/go-rovy"
)

const (
	NoiseConstruction = "Noise_IKpsk2_25519_ChaChaPoly_BLAKE2s"
	RovyIdentifier    = "rovy.net/session v0"
)

var (
	initialChainKey [blake2s.Size]byte
	initialHash     [blake2s.Size]byte
	zeroNonce       [chacha20poly1305.NonceSize]byte

	ErrZeroECDH = fmt.Errorf("zero result from ECDH")
	ErrAEADOpen = fmt.Errorf("aead open failed")
)

func init() {
	initialChainKey = blake2s.Sum256([]byte(NoiseConstruction))
	mixHash(&initialHash, &initialChainKey, []byte(RovyIdentifier))
}

type HelloHeader struct {
	Ephemeral rovy.PublicKey
	Static    [rovy.PublicKeySize + poly1305.TagSize]byte
	Timestamp [tai64n.TimestampSize + poly1305.TagSize]byte
	// MAC1      [blake2s.Size128]byte
	// MAC2      [blake2s.Size128]byte
}

type ResponseHeader struct {
	Ephemeral rovy.PublicKey
	Empty     [poly1305.TagSize]byte
	// MAC1      [blake2s.Size128]byte
	// MAC2      [blake2s.Size128]byte
}

type MessageHeader struct {
	Counter uint64
}

type Handshake struct {
	hash             [blake2s.Size]byte
	chainKey         [blake2s.Size]byte
	presharedKey     [chacha20poly1305.KeySize]byte
	localStatic      rovy.PrivateKey
	localEphemeral   rovy.PrivateKey
	remoteStatic     rovy.PublicKey
	remoteEphemeral  rovy.PublicKey
	precStaticStatic [rovy.PublicKeySize]byte
	sendKey          [chacha20poly1305.KeySize]byte
	receiveKey       [chacha20poly1305.KeySize]byte
	sendNonce        uint64
	initiator        bool
}

func NewHandshakeInitiator(localStatic rovy.PrivateKey, remoteStatic rovy.PublicKey) (*Handshake, error) {
	epriv, err := rovy.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	hs := &Handshake{
		initiator:        true,
		hash:             initialHash,
		chainKey:         initialChainKey,
		localStatic:      localStatic,
		localEphemeral:   epriv,
		remoteStatic:     remoteStatic,
		precStaticStatic: sharedSecret(localStatic, remoteStatic),
	}

	if isZero(hs.precStaticStatic[:]) {
		return nil, ErrZeroECDH
	}

	return hs, nil
}

func NewHandshakeResponder(localStatic rovy.PrivateKey) (*Handshake, error) {
	// TODO: create our ephemeral key only after we've successfully consumed the hello
	epriv, err := rovy.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	hs := &Handshake{
		initiator:      false,
		hash:           initialHash,
		chainKey:       initialChainKey,
		localStatic:    localStatic,
		localEphemeral: epriv,
	}

	return hs, nil
}

func (hs *Handshake) RemotePublicKey() rovy.PublicKey {
	return hs.remoteStatic
}

// TODO: encrypt payload2
func (hs *Handshake) MakeHello(payload []byte) (hdr HelloHeader, payload2 []byte, err error) {
	if !hs.initiator {
		err = fmt.Errorf("responder can't send hello")
		return
	}

	mixHash(&hs.hash, &hs.hash, hs.remoteStatic[:])

	hdr.Ephemeral = hs.localEphemeral.PublicKey()
	mixKey(&hs.chainKey, &hs.chainKey, hdr.Ephemeral[:])
	mixHash(&hs.hash, &hs.hash, hdr.Ephemeral[:])

	ss := sharedSecret(hs.localEphemeral, hs.remoteStatic)
	if isZero(ss[:]) {
		err = ErrZeroECDH
		return
	}

	var key [chacha20poly1305.KeySize]byte
	KDF2(&hs.chainKey, &key, hs.chainKey[:], ss[:])

	aead, err := chacha20poly1305.New(key[:])
	if err != nil {
		return
	}
	aead.Seal(hdr.Static[:0], zeroNonce[:], hs.localStatic.PublicKey().Bytes(), hs.hash[:])
	mixHash(&hs.hash, &hs.hash, hdr.Static[:])

	KDF2(&hs.chainKey, &key, hs.chainKey[:], hs.precStaticStatic[:])

	timestamp := tai64n.Now()
	aead, err = chacha20poly1305.New(key[:])
	if err != nil {
		return
	}
	aead.Seal(hdr.Timestamp[:0], zeroNonce[:], timestamp[:], hs.hash[:])
	mixHash(&hs.hash, &hs.hash, hdr.Timestamp[:])

	return
}

// TODO: encrypt payload2
// TODO: replay protection, flood protection
func (hs *Handshake) ConsumeHello(hdr HelloHeader, payload []byte) (payload2 []byte, err error) {
	if hs.initiator {
		err = fmt.Errorf("initiator can't consume hello")
		return
	}

	var hash [blake2s.Size]byte
	var chainKey [blake2s.Size]byte
	mixHash(&hash, &initialHash, hs.localStatic.PublicKey().Bytes())
	mixHash(&hash, &hash, hdr.Ephemeral[:])
	mixKey(&chainKey, &initialChainKey, hdr.Ephemeral[:])

	ss := sharedSecret(hs.localStatic, hdr.Ephemeral)
	if isZero(ss[:]) {
		err = ErrZeroECDH
		return
	}

	var key [chacha20poly1305.KeySize]byte
	KDF2(&chainKey, &key, chainKey[:], ss[:])

	var remoteStatic rovy.PublicKey
	aead, err := chacha20poly1305.New(key[:])
	if err != nil {
		return
	}
	_, err = aead.Open(remoteStatic[:0], zeroNonce[:], hdr.Static[:], hash[:])
	if err != nil {
		err = ErrAEADOpen
		return
	}
	mixHash(&hash, &hash, hdr.Static[:])

	precStaticStatic := sharedSecret(hs.localStatic, remoteStatic)
	if isZero(precStaticStatic[:]) {
		err = ErrZeroECDH
		return
	}
	KDF2(&chainKey, &key, chainKey[:], precStaticStatic[:])

	var timestamp tai64n.Timestamp
	aead, err = chacha20poly1305.New(key[:])
	if err != nil {
		return
	}
	_, err = aead.Open(timestamp[:0], zeroNonce[:], hdr.Timestamp[:], hash[:])
	if err != nil {
		err = ErrAEADOpen
		return
	}
	mixHash(&hash, &hash, hdr.Timestamp[:])

	hs.hash = hash
	hs.chainKey = chainKey
	hs.remoteStatic = remoteStatic
	hs.remoteEphemeral = hdr.Ephemeral

	return
}

func (hs *Handshake) MakeResponse(payload []byte) (hdr ResponseHeader, payload2 []byte, err error) {
	if hs.initiator {
		err = fmt.Errorf("initiator can't send response")
		return
	}

	hdr.Ephemeral = hs.localEphemeral.PublicKey()
	mixHash(&hs.hash, &hs.hash, hdr.Ephemeral[:])
	mixKey(&hs.chainKey, &hs.chainKey, hdr.Ephemeral[:])

	sse := sharedSecret(hs.localEphemeral, hs.remoteEphemeral)
	mixKey(&hs.chainKey, &hs.chainKey, sse[:])
	sss := sharedSecret(hs.localEphemeral, hs.remoteStatic)
	mixKey(&hs.chainKey, &hs.chainKey, sss[:])

	var tau [blake2s.Size]byte
	var key [chacha20poly1305.KeySize]byte
	KDF3(&hs.chainKey, &tau, &key, hs.chainKey[:], hs.presharedKey[:])
	mixHash(&hs.hash, &hs.hash, tau[:])

	aead, err := chacha20poly1305.New(key[:])
	if err != nil {
		return
	}
	aead.Seal(hdr.Empty[:0], zeroNonce[:], nil, hs.hash[:])
	mixHash(&hs.hash, &hs.hash, hdr.Empty[:])

	KDF2(&hs.receiveKey, &hs.sendKey, hs.chainKey[:], nil)

	return
}

func (hs *Handshake) ConsumeResponse(hdr ResponseHeader, payload []byte) (payload2 []byte, err error) {
	if !hs.initiator {
		err = fmt.Errorf("responder can't consume response")
		return
	}

	var hash [blake2s.Size]byte
	var chainKey [blake2s.Size]byte
	mixHash(&hash, &hs.hash, hdr.Ephemeral[:])
	mixKey(&chainKey, &hs.chainKey, hdr.Ephemeral[:])

	sse := sharedSecret(hs.localEphemeral, hdr.Ephemeral)
	mixKey(&chainKey, &chainKey, sse[:])
	sss := sharedSecret(hs.localStatic, hdr.Ephemeral)
	mixKey(&chainKey, &chainKey, sss[:])

	var tau [blake2s.Size]byte
	var key [chacha20poly1305.KeySize]byte
	KDF3(&chainKey, &tau, &key, chainKey[:], hs.presharedKey[:])
	mixHash(&hash, &hash, tau[:])

	aead, err := chacha20poly1305.New(key[:])
	if err != nil {
		return
	}
	_, err = aead.Open(nil, zeroNonce[:], hdr.Empty[:], hash[:])
	if err != nil {
		log.Printf("handshake: %+v", hs)
		// err = ErrAEADOpen
		return
	}
	mixHash(&hash, &hash, hdr.Empty[:])

	hs.hash = hash
	hs.chainKey = chainKey

	KDF2(&hs.sendKey, &hs.receiveKey, hs.chainKey[:], nil)

	return
}

func (hs *Handshake) MakeMessage() {}

func (hs *Handshake) ConsumeMessage() {}
