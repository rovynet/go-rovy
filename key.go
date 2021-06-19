package rovy

import (
	"bytes"
	"crypto/rand"
	"net"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/curve25519"
)

const (
	PrivateKeySize = 32
	PublicKeySize  = 32
)

var prefix = []byte{0xfc}

type PrivateKey struct {
	bytes [PrivateKeySize]byte
}

func NewPrivateKey(b []byte) PrivateKey {
	pk := PrivateKey{}
	copy(pk.bytes[:0], b)
	return pk
}

func GeneratePrivateKey() (PrivateKey, error) {
	var privkey PrivateKey
	_, err := rand.Read(privkey.bytes[:])
	if err != nil {
		return PrivateKey{}, err
	}

	privkey.clamp()

	ipv6 := privkey.PublicKey().Addr()
	if err != nil {
		return PrivateKey{}, err
	}
	if bytes.Equal(ipv6[:len(prefix)], prefix) {
		return privkey, nil
	}

	return GeneratePrivateKey()
}

func (privkey PrivateKey) clamp() {
	privkey.bytes[0] &= 248
	privkey.bytes[31] = (privkey.bytes[31] & 127) | 64
}

func (privkey PrivateKey) PublicKey() PublicKey {
	var pubkey PublicKey
	curve25519.ScalarBaseMult(&pubkey.bytes, &privkey.bytes)
	return pubkey
}

func (privkey PrivateKey) SharedSecret(remote PublicKey) [PublicKeySize]byte {
	var ss [PublicKeySize]byte
	curve25519.ScalarMult(&ss, &privkey.bytes, &remote.bytes)
	return ss
}

type PublicKey struct {
	bytes [PublicKeySize]byte
}

func NewPublicKey(b []byte) PublicKey {
	pubkey := PublicKey{}
	copy(pubkey.bytes[:], b)
	return pubkey
}

func (pubkey PublicKey) Bytes() []byte {
	return pubkey.bytes[:]
}

func (pubkey PublicKey) Addr() net.IP {
	hash := blake2s.Sum256(pubkey.bytes[:])
	h := blake2s.Sum256(hash[:])
	return net.IP(h[16:32])
}
