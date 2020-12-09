package ikpsk2

import (
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/poly1305"
	"golang.zx2c4.com/wireguard/tai64n"

	rovy "pkt.dev/go-rovy"
)

type HelloHeader struct {
	Ephemeral rovy.PublicKey
	Static    [rovy.PublicKeySize + poly1305.TagSize]byte
	Timestamp [tai64n.TimestampSize + poly1305.TagSize]byte
	MAC1      [blake2s.Size128]byte
	MAC2      [blake2s.Size128]byte
}

type ResponseHeader struct {
	Ephemeral rovy.PublicKey
	Empty     [poly1305.TagSize]byte
	MAC1      [blake2s.Size128]byte
	MAC2      [blake2s.Size128]byte
}

type MessageHeader struct {
	Counter uint64
}

type CookieHeader struct {
	Nonce  [chacha20poly1305.NonceSizeX]byte
	Cookie [blake2s.Size128 + poly1305.TagSize]byte
}
