package session

import (
	"encoding/binary"

	rovy "go.rovy.net"
)

var emptyTag [16]byte

//  4 bytes - msg type (0x1)
//  4 bytes - sender index
// 32 bytes - ephemeral key
// 48 bytes - static key + tag
// 28 bytes - timestamp + tag
//  .       - payload
// 16 bytes - payload tag
// = 132+ bytes
type HelloPacket struct {
	Offset  int
	Padding int
	rovy.Packet
}

func NewHelloPacket(basepkt rovy.Packet, offset, padding int) HelloPacket {
	pkt := HelloPacket{
		Packet:  basepkt,
		Offset:  offset,
		Padding: padding,
	}
	pkt.SetMsgType(HelloMsgType)
	return pkt
}

func (pkt HelloPacket) MsgType() uint32 {
	return binary.LittleEndian.Uint32(pkt.Buf[pkt.Offset+0 : pkt.Offset+4])
}

func (pkt HelloPacket) SetMsgType(msgt uint32) {
	binary.LittleEndian.PutUint32(pkt.Buf[pkt.Offset+0:pkt.Offset+4], msgt)
}

func (pkt HelloPacket) SenderIndex() uint32 {
	return binary.BigEndian.Uint32(pkt.Buf[pkt.Offset+4 : pkt.Offset+8])
}

func (pkt HelloPacket) SetSenderIndex(idx uint32) {
	binary.BigEndian.PutUint32(pkt.Buf[pkt.Offset+4:pkt.Offset+8], idx)
}

func (pkt HelloPacket) EphemeralKey() rovy.PublicKey {
	return rovy.NewPublicKey(pkt.Buf[pkt.Offset+8 : pkt.Offset+40])
}

func (pkt HelloPacket) SetEphemeralKey(key rovy.PublicKey) {
	copy(pkt.Buf[pkt.Offset+8:pkt.Offset+40], key.Bytes())
}

func (pkt HelloPacket) StaticKey() [48]byte {
	var key [48]byte
	copy(key[:], pkt.Buf[pkt.Offset+40:pkt.Offset+88])
	return key
}

func (pkt HelloPacket) SetStaticKey(empty [48]byte) {
	copy(pkt.Buf[pkt.Offset+40:pkt.Offset+88], empty[:])
}

func (pkt HelloPacket) Timestamp() [28]byte {
	var ts [28]byte
	copy(ts[:], pkt.Buf[pkt.Offset+88:pkt.Offset+116])
	return ts
}

func (pkt HelloPacket) SetTimestamp(empty [28]byte) {
	copy(pkt.Buf[pkt.Offset+88:pkt.Offset+116], empty[:])
}

func (pkt HelloPacket) Plaintext() []byte {
	return pkt.Buf[pkt.Offset+116 : pkt.Length-pkt.Padding-16]
}

// TODO what if plaintext is too long
func (pkt HelloPacket) SetPlaintext(pt []byte) HelloPacket {
	pkt.Length = pkt.Offset + 116 + len(pt) + 16 + pkt.Padding
	copy(pkt.Buf[pkt.Offset+116:pkt.Length-pkt.Padding], pt) // XXX does this do what i think
	copy(pkt.Buf[pkt.Length-16:pkt.Length-pkt.Padding], emptyTag[:])
	return pkt
}

func (pkt HelloPacket) Ciphertext() []byte {
	return pkt.Buf[pkt.Offset+116 : pkt.Length-pkt.Padding]
}

func (pkt HelloPacket) SetCiphertext(ct []byte) HelloPacket {
	pkt.Length = pkt.Offset + 116 + len(ct) + pkt.Padding
	copy(pkt.Buf[pkt.Offset+116:pkt.Length-pkt.Padding], ct)
	return pkt
}

//  4 bytes - msg type (0x2)
//  4 bytes - sender index
//  4 bytes - session index
// 32 bytes - ephemeral key
// 16 bytes - empty + tag
//  .       - payload
// 16 bytes - payload tag
// = 76+ bytes
type ResponsePacket struct {
	Offset  int
	Padding int
	rovy.Packet
}

func NewResponsePacket(basepkt rovy.Packet, offset, padding int) ResponsePacket {
	pkt := ResponsePacket{
		Packet:  basepkt,
		Offset:  offset,
		Padding: padding,
	}
	pkt.SetMsgType(ResponseMsgType)
	return pkt
}

func (pkt ResponsePacket) MsgType() uint32 {
	return binary.LittleEndian.Uint32(pkt.Buf[pkt.Offset+0 : pkt.Offset+4])
}

func (pkt ResponsePacket) SetMsgType(msgt uint32) {
	binary.LittleEndian.PutUint32(pkt.Buf[pkt.Offset+0:pkt.Offset+4], msgt)
}

func (pkt ResponsePacket) SenderIndex() uint32 {
	return binary.BigEndian.Uint32(pkt.Buf[pkt.Offset+4 : pkt.Offset+8])
}

func (pkt ResponsePacket) SetSenderIndex(idx uint32) {
	binary.BigEndian.PutUint32(pkt.Buf[pkt.Offset+4:pkt.Offset+8], idx)
}

func (pkt ResponsePacket) SessionIndex() uint32 {
	return binary.BigEndian.Uint32(pkt.Buf[pkt.Offset+8 : pkt.Offset+12])
}

func (pkt ResponsePacket) SetSessionIndex(idx uint32) {
	binary.BigEndian.PutUint32(pkt.Buf[pkt.Offset+8:pkt.Offset+12], idx)
}

func (pkt ResponsePacket) EphemeralKey() rovy.PublicKey {
	return rovy.NewPublicKey(pkt.Buf[pkt.Offset+12 : pkt.Offset+44])
}

func (pkt ResponsePacket) SetEphemeralKey(key rovy.PublicKey) {
	copy(pkt.Buf[pkt.Offset+12:pkt.Offset+44], key.Bytes())
}

func (pkt ResponsePacket) Empty() [16]byte {
	var empty [16]byte
	copy(empty[:], pkt.Buf[pkt.Offset+44:pkt.Offset+60])
	return empty
}

func (pkt ResponsePacket) SetEmpty(empty [16]byte) {
	copy(pkt.Buf[pkt.Offset+44:pkt.Offset+60], empty[:])
}

func (pkt ResponsePacket) Plaintext() []byte {
	return pkt.Buf[pkt.Offset+60 : pkt.Length-pkt.Padding-16]
}

func (pkt ResponsePacket) SetPlaintext(pt []byte) ResponsePacket {
	pkt.Length = pkt.Offset + 60 + len(pt) + 16 + pkt.Padding
	copy(pkt.Buf[pkt.Offset+60:pkt.Length-pkt.Padding], pt)
	copy(pkt.Buf[pkt.Length-pkt.Padding-16:pkt.Length-pkt.Padding], emptyTag[:])
	return pkt
}

func (pkt ResponsePacket) Ciphertext() []byte {
	return pkt.Buf[pkt.Offset+60 : pkt.Length-pkt.Padding]
}

func (pkt ResponsePacket) SetCiphertext(ct []byte) ResponsePacket {
	pkt.Length = pkt.Offset + 60 + len(ct) + pkt.Padding
	copy(pkt.Buf[pkt.Offset+60:pkt.Length-pkt.Padding], ct)
	return pkt
}

//  4 bytes - msg type (0x4)
//  4 bytes - session index
//  8 bytes - nonce
//  .       - payload
// 16 bytes - payload tag
// = 32+ bytes
type DataPacket struct {
	Offset  int
	Padding int
	rovy.Packet
}

func NewDataPacket(basepkt rovy.Packet, offset, padding int) DataPacket {
	pkt := DataPacket{
		Packet:  basepkt,
		Offset:  offset,
		Padding: padding,
	}
	pkt.SetMsgType(DataMsgType)
	return pkt
}

func (pkt DataPacket) MsgType() uint32 {
	return binary.LittleEndian.Uint32(pkt.Buf[pkt.Offset+0 : pkt.Offset+4])
}

func (pkt DataPacket) SetMsgType(msgt uint32) {
	binary.LittleEndian.PutUint32(pkt.Buf[pkt.Offset+0:pkt.Offset+4], msgt)
}

func (pkt DataPacket) SessionIndex() uint32 {
	return binary.BigEndian.Uint32(pkt.Buf[pkt.Offset+4 : pkt.Offset+8])
}

func (pkt DataPacket) SetSessionIndex(idx uint32) {
	binary.BigEndian.PutUint32(pkt.Buf[pkt.Offset+4:pkt.Offset+8], idx)
}

func (pkt DataPacket) Nonce() [8]byte {
	var nonce [8]byte
	copy(nonce[:], pkt.Buf[pkt.Offset+8:pkt.Offset+16])
	return nonce
}

func (pkt DataPacket) SetNonce(nonce [8]byte) {
	copy(pkt.Buf[pkt.Offset+8:pkt.Offset+16], nonce[:])
}

func (pkt DataPacket) Plaintext() []byte {
	return pkt.Buf[pkt.Offset+16 : pkt.Length-pkt.Padding-16]
}

func (pkt DataPacket) SetPlaintext(pt []byte) DataPacket {
	pkt.Length = pkt.Offset + 16 + len(pt) + 16 + pkt.Padding
	copy(pkt.Buf[pkt.Offset+16:pkt.Length-pkt.Padding], pt)
	copy(pkt.Buf[pkt.Length-pkt.Padding-16:pkt.Length-pkt.Padding], emptyTag[:])
	return pkt
}

func (pkt DataPacket) Ciphertext() []byte {
	return pkt.Buf[pkt.Offset+16 : pkt.Length-pkt.Padding]
}

func (pkt DataPacket) SetCiphertext(ct []byte) DataPacket {
	pkt.Length = pkt.Offset + 16 + len(ct) + pkt.Padding
	copy(pkt.Buf[pkt.Offset+16:pkt.Length-pkt.Padding], ct)
	return pkt
}

// XXX who knows if the following makes any sense lol

const PlaintextPacketSize = 4 + SignatureSize + RandomizerSize + rovy.PublicKeySize

const SignatureSize = 8
const RandomizerSize = 8

var stubSignature = [8]byte{0x42, 0x42, 0x42, 0x42, 0x42, 0x42, 0x42, 0x42}
var stubRandomizer = [8]byte{0x23, 0x23, 0x23, 0x23, 0x23, 0x23, 0x23, 0x23}

//  4 bytes - msg type (0x5)
//  8 bytes - signature
//  8 bytes - randomizer
// 32 bytes - sender static key
//  .       - data
// = 52+ bytes
type PlaintextPacket struct {
	Offset  int
	Padding int
	rovy.Packet
}

func NewPlaintextPacket(basepkt rovy.Packet, offset, padding int) PlaintextPacket {
	pkt := PlaintextPacket{
		Packet:  basepkt,
		Offset:  offset,
		Padding: padding,
	}
	pkt.SetMsgType(PlaintextMsgType)
	pkt.SetSignature(stubSignature)
	pkt.SetRandomizer(stubRandomizer)
	return pkt
}

func (pkt PlaintextPacket) MsgType() uint32 {
	o := pkt.Offset + 0
	return binary.LittleEndian.Uint32(pkt.Buf[o : o+4])
}

func (pkt PlaintextPacket) SetMsgType(msgt uint32) {
	o := pkt.Offset + 0
	binary.LittleEndian.PutUint32(pkt.Buf[o:o+4], msgt)
}

func (pkt PlaintextPacket) Signature() (sig [SignatureSize]byte) {
	o := pkt.Offset + 4
	copy(sig[:], pkt.Buf[o:o+SignatureSize])
	return sig
}

func (pkt PlaintextPacket) SetSignature(sig [SignatureSize]byte) {
	o := pkt.Offset + 4
	copy(pkt.Buf[o:o+SignatureSize], sig[:])
}

func (pkt PlaintextPacket) Randomizer() (rnd [RandomizerSize]byte) {
	o := pkt.Offset + 12
	copy(rnd[:], pkt.Buf[o:o+RandomizerSize])
	return rnd
}

func (pkt PlaintextPacket) SetRandomizer(rnd [RandomizerSize]byte) {
	o := pkt.Offset + 12
	copy(pkt.Buf[o:o+RandomizerSize], rnd[:])
}

func (pkt PlaintextPacket) Sender() rovy.PublicKey {
	o := pkt.Offset + 20
	return rovy.NewPublicKey(pkt.Buf[o : o+rovy.PublicKeySize])
}

func (pkt PlaintextPacket) SetSender(key rovy.PublicKey) {
	o := pkt.Offset + 20
	copy(pkt.Buf[o:o+rovy.PublicKeySize], key.Bytes())
}

func (pkt PlaintextPacket) Plaintext() []byte {
	o := pkt.Offset + 52
	return pkt.Buf[o : pkt.Length-pkt.Padding]
}

func (pkt PlaintextPacket) SetPlaintext(pt []byte) PlaintextPacket {
	o := pkt.Offset + 52
	pkt.Length = o + len(pt) + pkt.Padding
	copy(pkt.Buf[o:pkt.Length-pkt.Padding], pt)
	return pkt
}
