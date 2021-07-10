package rovy

import (
	"encoding/binary"

	multiaddr "github.com/multiformats/go-multiaddr"
)

const (
	LowerOffset = 0
	FwdOffset   = 16
	UpperOffset = 16 + 4 + 16
)

type Packet struct {
	Buf      []byte
	Length   int
	TptSrc   multiaddr.Multiaddr
	TptDst   multiaddr.Multiaddr
	LowerSrc PeerID
	LowerDst PeerID
	UpperSrc PeerID
	UpperDst PeerID
}

func NewPacket(buf []byte) Packet {
	return Packet{
		Buf:    buf,
		Length: len(buf),
	}
}

func (pkt Packet) Bytes() []byte {
	return pkt.Buf[:pkt.Length]
}

func (pkt Packet) MsgType() uint32 {
	return binary.LittleEndian.Uint32(pkt.Buf[0:4])
}

func (pkt Packet) SetMsgType(msgt uint32) {
	binary.LittleEndian.PutUint32(pkt.Buf[0:4], msgt)
}
