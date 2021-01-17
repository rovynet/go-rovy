package ikpsk2

import (
	"crypto/cipher"
	"encoding/binary"
	"fmt"

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
}

type ResponseHeader struct {
	Ephemeral rovy.PublicKey
	Empty     [poly1305.TagSize]byte
}

type MessageHeader struct {
	Nonce [8]byte
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
	send             cipher.AEAD
	receive          cipher.AEAD
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

// TODO: create our ephemeral key only after we've successfully consumed the hello
func NewHandshakeResponder(localStatic rovy.PrivateKey) (*Handshake, error) {
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

func (hs *Handshake) MakeHello(payload []byte) (HelloHeader, []byte, error) {
	hdr := HelloHeader{}

	if !hs.initiator {
		return hdr, nil, fmt.Errorf("responder can't send hello")
	}

	mixHash(&hs.hash, &hs.hash, hs.remoteStatic[:])

	hdr.Ephemeral = hs.localEphemeral.PublicKey()
	mixKey(&hs.chainKey, &hs.chainKey, hdr.Ephemeral[:])
	mixHash(&hs.hash, &hs.hash, hdr.Ephemeral[:])

	ss := sharedSecret(hs.localEphemeral, hs.remoteStatic)
	if isZero(ss[:]) {
		return hdr, nil, ErrZeroECDH
	}

	var key [chacha20poly1305.KeySize]byte
	KDF2(&hs.chainKey, &key, hs.chainKey[:], ss[:])

	aead, err := chacha20poly1305.New(key[:])
	if err != nil {
		return hdr, nil, err
	}
	aead.Seal(hdr.Static[:0], zeroNonce[:], hs.localStatic.PublicKey().Bytes(), hs.hash[:])
	mixHash(&hs.hash, &hs.hash, hdr.Static[:])

	KDF2(&hs.chainKey, &key, hs.chainKey[:], hs.precStaticStatic[:])

	timestamp := tai64n.Now()
	aead, err = chacha20poly1305.New(key[:])
	if err != nil {
		return hdr, nil, err
	}
	aead.Seal(hdr.Timestamp[:0], zeroNonce[:], timestamp[:], hs.hash[:])
	mixHash(&hs.hash, &hs.hash, hdr.Timestamp[:])

	return hdr, nil, nil
}

// TODO: replay protection
// TODO: flood protection
// TODO: cookies
func (hs *Handshake) ConsumeHello(hdr HelloHeader, payload []byte) ([]byte, error) {
	if hs.initiator {
		return nil, fmt.Errorf("initiator can't consume hello")
	}

	var hash [blake2s.Size]byte
	var chainKey [blake2s.Size]byte
	mixHash(&hash, &initialHash, hs.localStatic.PublicKey().Bytes())
	mixHash(&hash, &hash, hdr.Ephemeral[:])
	mixKey(&chainKey, &initialChainKey, hdr.Ephemeral[:])

	ss := sharedSecret(hs.localStatic, hdr.Ephemeral)
	if isZero(ss[:]) {
		return nil, ErrZeroECDH
	}

	var key [chacha20poly1305.KeySize]byte
	KDF2(&chainKey, &key, chainKey[:], ss[:])

	var remoteStatic rovy.PublicKey
	aead, err := chacha20poly1305.New(key[:])
	if err != nil {
		return nil, err
	}
	_, err = aead.Open(remoteStatic[:0], zeroNonce[:], hdr.Static[:], hash[:])
	if err != nil {
		return nil, ErrAEADOpen
	}
	mixHash(&hash, &hash, hdr.Static[:])

	precStaticStatic := sharedSecret(hs.localStatic, remoteStatic)
	if isZero(precStaticStatic[:]) {
		return nil, ErrZeroECDH
	}
	KDF2(&chainKey, &key, chainKey[:], precStaticStatic[:])

	var timestamp tai64n.Timestamp
	aead, err = chacha20poly1305.New(key[:])
	if err != nil {
		return nil, err
	}
	_, err = aead.Open(timestamp[:0], zeroNonce[:], hdr.Timestamp[:], hash[:])
	if err != nil {
		return nil, ErrAEADOpen
	}
	mixHash(&hash, &hash, hdr.Timestamp[:])

	hs.hash = hash
	hs.chainKey = chainKey
	hs.remoteStatic = remoteStatic
	hs.remoteEphemeral = hdr.Ephemeral

	return nil, nil
}

func (hs *Handshake) MakeResponse(payload []byte) (ResponseHeader, []byte, error) {
	hdr := ResponseHeader{}

	if hs.initiator {
		return hdr, nil, fmt.Errorf("initiator can't send response")
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
		return hdr, nil, err
	}
	aead.Seal(hdr.Empty[:0], zeroNonce[:], nil, hs.hash[:])
	mixHash(&hs.hash, &hs.hash, hdr.Empty[:])

	var sendKey [chacha20poly1305.KeySize]byte
	var recvKey [chacha20poly1305.KeySize]byte
	KDF2(&recvKey, &sendKey, hs.chainKey[:], nil)
	hs.send, err = chacha20poly1305.New(sendKey[:])
	if err != nil {
		return hdr, nil, err
	}
	hs.receive, err = chacha20poly1305.New(recvKey[:])
	if err != nil {
		return hdr, nil, err
	}

	return hdr, nil, nil
}

func (hs *Handshake) ConsumeResponse(hdr ResponseHeader, payload []byte) ([]byte, error) {
	if !hs.initiator {
		return nil, fmt.Errorf("responder can't consume response")
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
		return nil, err
	}
	_, err = aead.Open(nil, zeroNonce[:], hdr.Empty[:], hash[:])
	if err != nil {
		err = ErrAEADOpen
		return nil, err
	}
	mixHash(&hash, &hash, hdr.Empty[:])

	hs.hash = hash
	hs.chainKey = chainKey

	var sendKey [chacha20poly1305.KeySize]byte
	var recvKey [chacha20poly1305.KeySize]byte
	KDF2(&sendKey, &recvKey, hs.chainKey[:], nil)
	hs.send, err = chacha20poly1305.New(sendKey[:])
	if err != nil {
		return nil, err
	}
	hs.receive, err = chacha20poly1305.New(recvKey[:])
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (hs *Handshake) MakeMessage(payload []byte) (hdr MessageHeader, payload2 []byte, err error) {
	// see https://mailarchive.ietf.org/arch/msg/cfrg/u734TEOSDDWyQgE0pmhxjdncwvw/
	// padding := calculatePaddingSize(len(payload), rovy.PreliminaryMTU-12)
	// for i := 0; i < padding; i++ {
	// 	payload = append(payload, 0x0)
	// }

	hs.sendNonce += 1
	binary.BigEndian.PutUint64(hdr.Nonce[:], hs.sendNonce)

	// XXX: why leave the first 4 bytes zero instead of some other part?
	// TODO: use copy(nonce[4:], hdr.Nonce) instead
	var nonce [chacha20poly1305.NonceSize]byte
	nonce[0x4] = hdr.Nonce[0x0]
	nonce[0x5] = hdr.Nonce[0x1]
	nonce[0x6] = hdr.Nonce[0x2]
	nonce[0x7] = hdr.Nonce[0x3]
	nonce[0x8] = hdr.Nonce[0x4]
	nonce[0x9] = hdr.Nonce[0x5]
	nonce[0xa] = hdr.Nonce[0x6]
	nonce[0xb] = hdr.Nonce[0x7]
	payload2 = hs.send.Seal(payload2[:0], nonce[:], payload, nil)

	return
}

func (hs *Handshake) ConsumeMessage(hdr MessageHeader, payload []byte) (payload2 []byte, err error) {
	var nonce [chacha20poly1305.NonceSize]byte
	nonce[0x4] = hdr.Nonce[0x0]
	nonce[0x5] = hdr.Nonce[0x1]
	nonce[0x6] = hdr.Nonce[0x2]
	nonce[0x7] = hdr.Nonce[0x3]
	nonce[0x8] = hdr.Nonce[0x4]
	nonce[0x9] = hdr.Nonce[0x5]
	nonce[0xa] = hdr.Nonce[0x6]
	nonce[0xb] = hdr.Nonce[0x7]
	payload2, err = hs.receive.Open(payload2[:0], nonce[:], payload, nil)

	return
}
