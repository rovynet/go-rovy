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
	// 0x73 comes right after the "libp2p-key" multicodec at 0x72.
	//
	// TODO: see if there's a number that gets us 'brovyai'
	// TODO: pick a number that works as a multiaddr too
	RovyKeyMulticodec = 0x73

	RovyMultiaddrCodec = 0x1a6

	PreliminaryMTU = 1500
	// PreliminaryMTU = 500
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

type PeerID cid.Cid

func NewPeerID(pubkey PublicKey) PeerID {
	bytes := pubkey[:]
	mhash, _ := multihash.Sum(bytes, multihash.IDENTITY, len(bytes))
	return PeerID(cid.NewCidV1(RovyKeyMulticodec, mhash))
}

func (pid PeerID) Bytes() []byte {
	return cid.Cid(pid).Bytes()
}

func (pid PeerID) String() string {
	return cid.Cid(pid).String()
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
	return PeerID(c).String(), nil
}

func maddrValid(b []byte) error {
	_, err := cid.Cast(b)
	return err
}
