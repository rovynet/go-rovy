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

type (
	PrivateKey [PrivateKeySize]byte
	PublicKey  [PublicKeySize]byte
)

func NewPrivateKey() (PrivateKey, error) {
	var privkey PrivateKey
	_, err := rand.Read(privkey[:])
	privkey.clamp()
	ipv6 := privkey.PublicKey().Addr()
	if err != nil || bytes.Equal(ipv6[:len(prefix)], prefix) {
		return privkey, err
	}
	return NewPrivateKey()
}

func (privkey *PrivateKey) clamp() {
	privkey[0] &= 248
	privkey[31] = (privkey[31] & 127) | 64
}

func (privkey *PrivateKey) PublicKey() PublicKey {
	var pubkey PublicKey
	apubk := (*[PublicKeySize]byte)(&pubkey)
	aprivk := (*[PrivateKeySize]byte)(privkey)
	curve25519.ScalarBaseMult(apubk, aprivk)
	return pubkey
}

func (pubkey PublicKey) Bytes() []byte {
	return pubkey[:]
}

func (pubkey PublicKey) Addr() net.IP {
	hash := blake2s.Sum256(pubkey[:])
	h := blake2s.Sum256(hash[:])
	return net.IP(h[16:32])
}
