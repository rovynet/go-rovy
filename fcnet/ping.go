package fcnet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"

	icmp "golang.org/x/net/icmp"
	ipv6 "golang.org/x/net/ipv6"

	rovy "go.rovy.net"
)

const SignatureSize = 8
const RandomizerSize = 8
const RequestIdSize = 4

var stubSignature = [SignatureSize]byte{0x1, 0x3, 0x1, 0x2, 0x1, 0x3, 0x1, 0x2}
var stubRandomizer = [RandomizerSize]byte{0x42, 0x42, 0x42, 0x42, 0x42, 0x42, 0x42, 0x42}
var pingRequestId = [RequestIdSize]byte{0x0, 0x0, 0x0, 0x1}
var pongRequestId = [RequestIdSize]byte{0x0, 0x0, 0x0, 0x2}

// TODO implement me
func (fc *Fcnet) sign(ppkt PingPacket) (PingPacket, error) {
	return ppkt, nil
}

// TODO implement me
func (fc *Fcnet) verify(ppkt PingPacket) error {
	return nil
}

func (fc *Fcnet) handlePingPacket(lpkt rovy.LowerPacket) error {
	ppkt := NewPingPacket(lpkt.Packet)

	if !ppkt.IsDestination() {
		return fc.node.Forwarder().HandlePacket(lpkt)
	}

	prevrt, err := fc.node.Routing().GetRoute(lpkt.LowerSrc)
	if err != nil {
		return err
	}
	fwdhdr := ppkt.Buf[ppkt.Offset+4 : ppkt.Offset+20]
	fwdhdr[2+fwdhdr[0]] = prevrt.Bytes()[0]
	fwdhdr[0] = fwdhdr[0] + 1

	route := rovy.NewRoute(fwdhdr[2 : 2+fwdhdr[0]]...).Reverse()

	if ppkt.IsReply() {
		if err := fc.verify(ppkt); err != nil {
			return err
		}

		buf := ppkt.Payload()

		dst := fc.ip.AsSlice()
		if 0 != bytes.Compare(buf[8:24], dst) {
			return fmt.Errorf("fcnet: recv: dst address mismatch -- expected %#v -- got %#v", dst, buf[8:24])
		}

		src2 := ppkt.Sender().IPAddr().AsSlice()
		dst2 := dst

		body := &icmp.TimeExceeded{Data: buf[:]}
		msg := icmp.Message{
			Type: ipv6.ICMPTypeTimeExceeded,
			Code: 0,
			Body: body,
		}

		// about traceroute on fedora:
		//
		// firewalld drops these replies although they're fine.
		// it probably recognizes the hops as local interfaces
		// and partially blocks incoming cross-interface icmp.
		//
		// to verify, try traceroute non-local peerings: laptop->desktop->server
		// or temporarily shutdown firewalld.
		//
		chdr := icmp.IPv6PseudoHeader(src2, dst2)
		cbuf := append(chdr, 0x3, 0x0, 0, 0)
		cbody, err := msg.Body.Marshal(ipv6.ICMPTypeTimeExceeded.Protocol())
		if err != nil {
			return fmt.Errorf("fcnet: icmp checksuming error: %s", err)
		}
		cbuf = append(cbuf, cbody...)
		lenoff := 2 * net.IPv6len
		binary.BigEndian.PutUint32(cbuf[lenoff:lenoff+4], uint32(len(cbuf)-len(chdr)))
		csum := icmpChecksum(cbuf)
		cbuf[len(chdr)+2] ^= byte(csum)
		cbuf[len(chdr)+3] ^= byte(csum >> 8)
		icmpdata := cbuf[len(chdr):]

		// TODO: make sure the resulting packet doesn't exceed MTU
		ilen := len(icmpdata)
		p2 := make([]byte, 40+ilen)
		copy(p2[0:4], buf[0:4]) // copying the src flowlabel might be stupid
		binary.BigEndian.PutUint16(p2[4:6], uint16(ilen))
		p2[6] = 58
		p2[7] = 64
		copy(p2[8:24], src2)
		copy(p2[24:40], dst2)
		copy(p2[40:], icmpdata)

		return fc.handleFcnetPacket(rovy.NewPeerID(ppkt.Sender()), p2)
	}

	ppkt2 := NewPingPacket(rovy.NewPacket(make([]byte, rovy.TptMTU)))
	ppkt2.LowerSrc = fc.node.PeerID()
	ppkt2.SetRoute(route)
	ppkt2.SetSender(fc.node.PeerID().PublicKey())
	ppkt2.SetIsReply(true)
	ppkt2 = ppkt2.SetPayload(ppkt.Payload())

	ppkt2, err = fc.sign(ppkt2)
	if err != nil {
		return err
	}

	lpkt2 := rovy.NewLowerPacket(ppkt2.Packet)
	lpkt2.SetCodec(PingMulticodec)
	return fc.node.Forwarder().SendRaw(lpkt2)
}

// From golang.org/x/net/icmp
func icmpChecksum(b []byte) uint16 {
	csumcv := len(b) - 1 // checksum coverage
	s := uint32(0)
	for i := 0; i < csumcv; i += 2 {
		s += uint32(b[i+1])<<8 | uint32(b[i])
	}
	if csumcv&1 == 0 {
		s += uint32(b[csumcv])
	}
	s = s>>16 + s&0xffff
	s = s + s>>16
	return ^uint16(s)
}

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
