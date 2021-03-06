package rovy

import (
	"bytes"
	"crypto/rand"
	"net/netip"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/curve25519"

	multibase "github.com/multiformats/go-multibase"
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
	copy(pk.bytes[:32], b[:PrivateKeySize])
	return pk
}

func MustGeneratePrivateKey() PrivateKey {
	privkey, err := GeneratePrivateKey()
	if err != nil {
		panic(err)
	}
	return privkey
}

func GeneratePrivateKey() (PrivateKey, error) {
	var privkey PrivateKey
	_, err := rand.Read(privkey.bytes[:])
	if err != nil {
		return PrivateKey{}, err
	}

	privkey.clamp()

	ipv6 := privkey.PublicKey().IPAddr().As16()
	if err != nil {
		return PrivateKey{}, err
	}
	if bytes.Equal(ipv6[:len(prefix)], prefix) {
		return privkey, nil
	}

	return GeneratePrivateKey()
}

func ParsePrivateKey(str string) (PrivateKey, error) {
	_, b, err := multibase.Decode(str)
	if err != nil {
		return PrivateKey{}, err
	}
	return NewPrivateKey(b), nil
}

func MustParsePrivateKey(str string) PrivateKey {
	privkey, err := ParsePrivateKey(str)
	if err != nil {
		panic(err)
	}
	return privkey
}

func (privkey PrivateKey) clamp() {
	privkey.bytes[0] &= 248
	privkey.bytes[31] = (privkey.bytes[31] & 127) | 64
}

func (privkey PrivateKey) Bytes() [PrivateKeySize]byte {
	return privkey.bytes
}

func (privkey PrivateKey) BytesSlice() []byte {
	return privkey.bytes[:]
}

func (privkey PrivateKey) String() string {
	return multibase.MustNewEncoder(multibase.Base64).Encode(privkey.bytes[:])
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

func (pubkey PublicKey) IPAddr() netip.Addr {
	h1 := blake2s.Sum256(pubkey.bytes[:])
	h2 := blake2s.Sum256(h1[:])
	ip, _ := netip.AddrFromSlice(h2[16:32])
	return ip
}
