package rovy

import (
	"encoding/binary"
	"log"

	varint "github.com/multiformats/go-varint"
)

const (
	LowerOffset  = 0
	LowerPadding = 0
	FwdOffset    = 20
	UpperOffset  = 16 + 4 + 16
	UpperPadding = 16

	// all of rovy's layers combined
	UpperMTU = 1500 - 48 - 20 - 16 - 20 - 16 - 16 // 1364

	// ethernet + udp6
	TptMTU = 1500 - 48
)

type Allocator interface {
	AllocatePacket() (Packet, error)
}

type Packet struct {
	Buf       []byte // XXX should be private
	Length    int
	release   func()
	available *bool

	TptSrc   Multiaddr
	TptDst   Multiaddr
	LowerSrc PeerID
	LowerDst PeerID
	UpperSrc PeerID
	UpperDst PeerID
}

func NewPacket(buf []byte, release func()) Packet {
	av := true
	pkt := Packet{
		Buf:       buf,
		Length:    len(buf),
		release:   release,
		available: &av,
	}
	return pkt
}

func (pkt Packet) Release() {
	if !*pkt.available {
		panic("packet already released")
	}
	av := false
	pkt.available = &av
	if pkt.release != nil {
		pkt.release()
	}
}

func (pkt Packet) Bytes() []byte {
	if !*pkt.available {
		panic("packet already released")
	}
	return pkt.Buf[:pkt.Length]
}

func (pkt Packet) MsgType() uint32 {
	return binary.LittleEndian.Uint32(pkt.Buf[0:4])
}

func (pkt Packet) SetMsgType(msgt uint32) {
	binary.LittleEndian.PutUint32(pkt.Buf[0:4], msgt)
}

type UpperPacket struct {
	Offset  int
	Padding int
	Packet
}

func NewUpperPacket(basepkt Packet) UpperPacket {
	return UpperPacket{
		Packet: basepkt,
		Offset: UpperOffset + 16, // msgtype+index+nonce
		// we're the plaintext upper packet, need room for our own tag, and the lower tag
		Padding: 32,
	}
}

// TODO max length 2+14 bytes
func (pkt UpperPacket) Route() Route {
	o := FwdOffset
	length := int(pkt.Buf[o+1])
	return NewRoute(pkt.Buf[o+2 : o+2+length]...)
}

func (pkt UpperPacket) RouteLen() int {
	return int(pkt.Buf[FwdOffset+1])
}

// TODO max length 2+14 bytes
func (pkt UpperPacket) SetRoute(rt Route) {
	o := FwdOffset
	length := rt.Len()
	pkt.Buf[o+0] = 0x0
	pkt.Buf[o+1] = byte(length)
	copy(pkt.Buf[o+2:o+2+length], rt.Bytes())
	for i := 2 + length; i < 16; i++ {
		pkt.Buf[o+i] = 0x0
	}
}

func (pkt UpperPacket) Codec() (uint64, error) {
	o := pkt.Offset + 0
	codec, _, err := varint.FromUvarint(pkt.Buf[o+0 : o+4])
	return codec, err
}

func (pkt UpperPacket) SetCodec(codec uint64) {
	o := pkt.Offset + 0
	buf := varint.ToUvarint(codec)
	if len(buf) > 4 {
		log.Panicf("varint too large for 4 bytes: %#v", buf)
	}
	copy(pkt.Buf[o+0:o+4], buf)
}

func (pkt UpperPacket) Payload() []byte {
	o := pkt.Offset + 4
	return pkt.Buf[o : pkt.Length-pkt.Padding]
}

func (pkt UpperPacket) SetPayload(pl []byte) UpperPacket {
	o := pkt.Offset + 4
	pkt.Length = o + len(pl) + pkt.Padding
	copy(pkt.Buf[o:pkt.Length-pkt.Padding], pl)
	return pkt
}

type LowerPacket struct {
	Offset int
	Packet
}

func NewLowerPacket(basepkt Packet) LowerPacket {
	return LowerPacket{Packet: basepkt, Offset: 16}
}

func (pkt LowerPacket) Codec() (uint64, error) {
	o := pkt.Offset + 0
	codec, _, err := varint.FromUvarint(pkt.Buf[o+0 : o+4])
	return codec, err
}

func (pkt LowerPacket) SetCodec(codec uint64) {
	o := pkt.Offset + 0
	buf := varint.ToUvarint(codec)
	if len(buf) > 4 {
		log.Panicf("varint too large for 4 bytes: %#v", buf)
	}
	copy(pkt.Buf[o+0:o+4], buf)
}

func (pkt LowerPacket) Payload() []byte {
	o := pkt.Offset + 4
	return pkt.Buf[o : pkt.Length-16]
}

func (pkt LowerPacket) SetPayload(pl []byte) LowerPacket {
	o := pkt.Offset + 4
	pkt.Length = o + len(pl) + 16
	copy(pkt.Buf[o:pkt.Length-16], pl)
	return pkt
}
