package rovy

import (
	"fmt"

	cid "github.com/ipfs/go-cid"
	multiaddr "github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"
)

const (
	// RovyKeyMulticodec is the "rovy-key" multicodec header value.
	//
	// We're not using the "libp2p-key" multicodec, because that'd end up
	// with base32 PeerIDs of length 67, which is too long for a subdomain.
	// libp2p-key includes a whole protobuf structure wrapping the actual key,
	// which is data that we want to and can avoid. The "rovy-key" multicodec
	// simply uses the identity hash function and no additional wrapping,
	// so the multihash we end up putting in the PeerID is the public key data
	// itself, and nothing else. Including the rovy-key multicodec header,
	// that makes 59 bytes when base32-encoded, which fits into the 63 bytes
	// subdomain length limit. Unencoded, the PeerID is 36 bytes.
	//
	// The base32-encoded PeerID always starts with 'bafzaai'.
	//
	// 0x73 comes right after the "libp2p-key" multicodec at 0x72, we might get
	// asked to pick a higher number in double-byte-varint space.
	//
	// TODO: officially register the multicodec number
	RovyKeyMulticodec = 0x73

	// TODO: officially register the multicodec number
	RovyMultiaddrCodec = 0x1a6
)

func init() {
	cid.Codecs["rovy-key"] = RovyKeyMulticodec
	cid.CodecToStr[RovyKeyMulticodec] = "rovy-key"

	multiaddr.AddProtocol(multiaddr.Protocol{
		Name:       "rovy",
		Code:       RovyMultiaddrCodec,
		VCode:      multiaddr.CodeToVarint(RovyMultiaddrCodec),
		Size:       multiaddr.LengthPrefixedVarSize,
		Transcoder: multiaddr.NewTranscoderFromFunctions(maddrStr2b, maddrB2Str, maddrValid),
	})
}

var EmptyPeerID PeerID

type PeerID struct {
	b1 uint64
	b2 uint64
	b3 uint64
	b4 uint64
}

func NewPeerID(pk PublicKey) PeerID {
	b := pk.Bytes()
	return PeerID{
		b1: beUint64(b[:8]),
		b2: beUint64(b[8:16]),
		b3: beUint64(b[16:24]),
		b4: beUint64(b[24:]),
	}
}

func PeerIDFromCid(c cid.Cid) (PeerID, error) {
	if c.Type() != RovyKeyMulticodec {
		return PeerID{}, fmt.Errorf("peerid can't be cid with type %O", c.Type())
	}

	mhash, err := multihash.Decode(c.Hash())
	if err != nil {
		return PeerID{}, err
	}

	if mhash.Code != multihash.IDENTITY {
		return PeerID{}, fmt.Errorf("public key mishhashed as 0x%x", mhash.Code)
	}
	if mhash.Length != PublicKeySize {
		return PeerID{}, fmt.Errorf("invalid public key size: %d", mhash.Length)
	}

	return NewPeerID(NewPublicKey(mhash.Digest)), nil
}

func (pid PeerID) Bytes() []byte {
	return pid.Cid().Bytes()
}

func (pid PeerID) RawBytesTo(b []byte) {
	_ = b[31]
	bePutUint64(b[:8], pid.b1)
	bePutUint64(b[8:16], pid.b2)
	bePutUint64(b[16:24], pid.b3)
	bePutUint64(b[24:], pid.b4)
}

func (pid PeerID) Empty() bool {
	return pid == EmptyPeerID
}

func (pid PeerID) Equal(other PeerID) bool {
	return pid == other
}

func (pid PeerID) String() string {
	if pid.Empty() {
		return "<empty>"
	}
	return pid.Cid().String()
}

func (pid PeerID) PublicKey() PublicKey {
	var b [32]byte
	bePutUint64(b[:8], pid.b1)
	bePutUint64(b[8:16], pid.b2)
	bePutUint64(b[16:24], pid.b3)
	bePutUint64(b[24:], pid.b4)
	return NewPublicKey(b[:])
}

func (pid PeerID) Cid() cid.Cid {
	mhash, _ := multihash.Sum(pid.PublicKey().Bytes(), multihash.IDENTITY, PublicKeySize)
	return cid.NewCidV1(0x73, mhash)
}

// from stdlib net/netip/leaf_alts.go
func beUint64(b []byte) uint64 {
	_ = b[7] // bounds check hint to compiler; see golang.org/issue/14808
	return uint64(b[7]) | uint64(b[6])<<8 | uint64(b[5])<<16 | uint64(b[4])<<24 |
		uint64(b[3])<<32 | uint64(b[2])<<40 | uint64(b[1])<<48 | uint64(b[0])<<56
}

// from stdlib net/netip/leaf_alts.go
func bePutUint64(b []byte, v uint64) {
	_ = b[7] // early bounds check to guarantee safety of writes below
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
}

func maddrStr2b(s string) ([]byte, error) {
	c, err := cid.Decode(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rovy addr: '%s' %s", s, err)
	}

	if ty := c.Type(); ty == RovyKeyMulticodec {
		return c.Bytes(), nil
	} else {
		return nil, fmt.Errorf("failed to parse rovy addr: '%s' has invalid codec %d", s, ty)
	}
}

func maddrB2Str(b []byte) (string, error) {
	c, err := cid.Cast(b)
	if err != nil {
		return "", err
	}
	pid, err := PeerIDFromCid(c)
	if err != nil {
		return "", err
	}
	return pid.String(), nil
}

func maddrValid(b []byte) error {
	_, err := cid.Cast(b)
	return err
}
