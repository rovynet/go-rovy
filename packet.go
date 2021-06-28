package rovy

// TODO: check this out: https://github.com/lunixbochs/struc
// TODO: figure out endianness once and for all

import (
	"encoding/binary"

	multiaddr "github.com/multiformats/go-multiaddr"
)

type Packet struct {
	Bytes       []byte
	TptSrc      multiaddr.Multiaddr
	TptDst      multiaddr.Multiaddr
	LowerOffset uint16
	UpperOffset uint16
	LowerSrc    PeerID
	LowerDst    PeerID
	UpperSrc    PeerID
	UpperDst    PeerID
}

func (pkt Packet) MsgType() uint32 {
	return binary.LittleEndian.Uint32(pkt.Bytes[0:4])
}
