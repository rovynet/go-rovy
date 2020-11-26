package rovy

import (
	"crypto/rand"
	"golang.org/x/crypto/curve25519"
)

const (
	PrivateKeySize = 32
	PublicKeySize  = 32
)

type (
	PrivateKey [PrivateKeySize]byte
	PublicKey  [PublicKeySize]byte
)

func NewPrivateKey() (PrivateKey, error) {
	var privkey PrivateKey
	_, err := rand.Read(privkey[:])
	privkey.clamp()
	return privkey, err
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
