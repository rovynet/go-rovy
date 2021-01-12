package rovy

import (
	"bytes"
	"encoding/binary"
	"fmt"

	cid "github.com/ipfs/go-cid"
	multiaddr "github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"
	varint "github.com/multiformats/go-varint"
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

	PreliminaryMTU = 1500
	// PreliminaryMTU = 500

	MaxPeerIDSize = 128
)

var (
	NullPeerID = PeerID([32]byte{})
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

type PeerID PublicKey

func NewPeerID(pubkey PublicKey) PeerID {
	return PeerID(pubkey)
}

func NewPeerIDFromCid(c cid.Cid) (pid PeerID, err error) {
	if c.Type() != RovyKeyMulticodec {
		err = fmt.Errorf("peerid can't be cid with type %O", c.Type())
		return
	}

	mhash, err := multihash.Decode(c.Hash())
	if err != nil {
		return
	}

	if mhash.Length != PublicKeySize {
		err = fmt.Errorf("invalid public key size: %d", mhash.Length)
		return
	}

	copy(pid[:], mhash.Digest)
	return
}

func Buf2PeerID(r *bytes.Buffer) (pid PeerID, err error) {
	size, err := varint.ReadUvarint(r)
	if err != nil {
		return
	}
	if size > MaxPeerIDSize {
		err = fmt.Errorf("PeerID too long")
		return
	}
	bytes := make([]byte, size)
	if err = binary.Read(r, binary.BigEndian, bytes); err != nil {
		return
	}
	_, c, err := cid.CidFromBytes(bytes[:size])
	if err != nil {
		return
	}
	return NewPeerIDFromCid(c)
}

// TODO: double-check the size calculation
func PeerID2Buf(pid PeerID, w *bytes.Buffer) (err error) {
	bytes := pid.Bytes()
	size := varint.ToUvarint(uint64(binary.Size(bytes)))
	if err = binary.Write(w, binary.BigEndian, size); err != nil {
		return
	}
	return binary.Write(w, binary.BigEndian, bytes)
}

func (pid PeerID) cid() cid.Cid {
	mhash, _ := multihash.Sum(pid[:], multihash.IDENTITY, PublicKeySize)
	return cid.NewCidV1(RovyKeyMulticodec, mhash)
}

func (pid PeerID) Bytes() []byte {
	return pid.cid().Bytes()
}

func (pid PeerID) String() string {
	return pid.cid().String()
}

func (pid PeerID) Equal(other PeerID) bool {
	return bytes.Equal(pid.Bytes(), other.Bytes())
}

func (pid PeerID) PublicKey() PublicKey {
	return PublicKey(pid)
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
	pid, err := NewPeerIDFromCid(c)
	if err != nil {
		return "", err
	}
	return pid.String(), nil
}

func maddrValid(b []byte) error {
	_, err := cid.Cast(b)
	return err
}
