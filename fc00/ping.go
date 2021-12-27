package fc00

import (
	"bytes"

	rovy "go.rovy.net"
)

const SignatureSize = 8
const RandomizerSize = 8
const RequestIdSize = 4

var stubSignature = [SignatureSize]byte{0x1, 0x3, 0x1, 0x2, 0x1, 0x3, 0x1, 0x2}
var stubRandomizer = [RandomizerSize]byte{0x42, 0x42, 0x42, 0x42, 0x42, 0x42, 0x42, 0x42}
var pingRequestId = [RequestIdSize]byte{0x0, 0x0, 0x0, 0x1}
var pongRequestId = [RequestIdSize]byte{0x0, 0x0, 0x0, 0x2}

//  4 bytes - codec
// 16 bytes - forwarder header
// 32 bytes - sender static key
//  8 bytes - signature (over requestID+randomizer+payload)
//  4 bytes   request id
//  8 bytes - randomizer
//  .       - payload
// = 72+ bytes
type PingPacket struct {
	Offset  int
	Padding int
	rovy.Packet
}

func NewPingPacket(basepkt rovy.Packet) PingPacket {
	pkt := PingPacket{
		Packet:  basepkt,
		Offset:  16, // msgtype + session index + nonce
		Padding: 16,
	}

	pkt.SetSignature(stubSignature)
	pkt.SetRandomizer(stubRandomizer)
	return pkt
}

func (pkt PingPacket) IsReply() bool {
	reqid := pkt.RequestId()
	return bytes.Equal(reqid[:], pongRequestId[:])
}

func (pkt PingPacket) SetIsReply(is bool) {
	if is {
		pkt.SetRequestId(pongRequestId)
	} else {
		pkt.SetRequestId(pingRequestId)
	}
}

// TODO max length 2+14 bytes
func (pkt PingPacket) Route() rovy.Route {
	o := pkt.Offset + 4
	length := int(pkt.Buf[o+1])
	return rovy.NewRoute(pkt.Buf[o+2 : o+2+length]...)
}

// TODO max length 2+14 bytes
func (pkt PingPacket) SetRoute(rt rovy.Route) {
	o := pkt.Offset + 4
	length := rt.Len()
	pkt.Buf[o+0] = 0x0
	pkt.Buf[o+1] = byte(length)
	copy(pkt.Buf[o+2:o+2+length], rt.Bytes())
	for i := 2 + length; i < 16; i++ {
		pkt.Buf[o+i] = 0x0
	}
}

func (pkt PingPacket) IsDestination() bool {
	o := pkt.Offset + 4
	return int(pkt.Buf[o])+1 >= int(pkt.Buf[o+1])
}

func (pkt PingPacket) Sender() rovy.PublicKey {
	o := pkt.Offset + 20
	return rovy.NewPublicKey(pkt.Buf[o : o+rovy.PublicKeySize])
}

func (pkt PingPacket) SetSender(key rovy.PublicKey) {
	o := pkt.Offset + 20
	copy(pkt.Buf[o:o+rovy.PublicKeySize], key.Bytes())
}

func (pkt PingPacket) Signature() (sig [SignatureSize]byte) {
	o := pkt.Offset + 52
	copy(sig[:], pkt.Buf[o:o+SignatureSize])
	return sig
}

func (pkt PingPacket) SetSignature(sig [SignatureSize]byte) {
	o := pkt.Offset + 52
	copy(pkt.Buf[o:o+SignatureSize], sig[:])
}

func (pkt PingPacket) RequestId() (reqid [RequestIdSize]byte) {
	o := pkt.Offset + 60
	copy(reqid[:], pkt.Buf[o:o+RequestIdSize])
	return reqid
}

func (pkt PingPacket) SetRequestId(reqid [RequestIdSize]byte) {
	o := pkt.Offset + 60
	copy(pkt.Buf[o:o+RequestIdSize], reqid[:])
}

func (pkt PingPacket) Randomizer() (rnd [RandomizerSize]byte) {
	o := pkt.Offset + 64
	copy(rnd[:], pkt.Buf[o:o+RandomizerSize])
	return rnd
}

func (pkt PingPacket) SetRandomizer(rnd [RandomizerSize]byte) {
	o := pkt.Offset + 64
	copy(pkt.Buf[o:o+RandomizerSize], rnd[:])
}

func (pkt PingPacket) Payload() []byte {
	o := pkt.Offset + 72
	return pkt.Buf[o : pkt.Length-pkt.Padding]
}

func (pkt PingPacket) SetPayload(pt []byte) PingPacket {
	o := pkt.Offset + 72
	pkt.Length = o + len(pt) + pkt.Padding
	copy(pkt.Buf[o:pkt.Length-pkt.Padding], pt)
	return pkt
}
