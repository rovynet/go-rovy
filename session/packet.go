package session

import (
	"bytes"
	"encoding/binary"

	varint "github.com/multiformats/go-varint"

	rovy "pkt.dev/go-rovy"
	ikpsk2 "pkt.dev/go-rovy/session/ikpsk2"
)

const HelloPacketSize = 4 + 4 + ikpsk2.HelloHeaderSize

type HelloPacket struct {
	MsgType     uint32
	SenderIndex uint32
	ikpsk2.HelloHeader
	Payload []byte
}

func (pkt *HelloPacket) MarshalBinary() ([]byte, error) {
	payloadSize := varint.ToUvarint(uint64(binary.Size(pkt.Payload)))

	buf := make([]byte, HelloPacketSize+len(payloadSize)+len(pkt.Payload))
	w := bytes.NewBuffer(buf[:0])

	if err := binary.Write(w, binary.LittleEndian, pkt.MsgType); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.SenderIndex); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.HelloHeader.Ephemeral); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.HelloHeader.Static); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.HelloHeader.Timestamp); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, payloadSize); err != nil {
		return buf[:], err
	}
	if err := binary.Write(w, binary.BigEndian, pkt.Payload); err != nil {
		return buf[:], err
	}

	return buf[:], nil
}

func (pkt *HelloPacket) UnmarshalBinary(buf []byte) (err error) {
	r := bytes.NewBuffer(buf)

	if err = binary.Read(r, binary.LittleEndian, &pkt.MsgType); err != nil {
		return err
	}

	if err = binary.Read(r, binary.BigEndian, &pkt.SenderIndex); err != nil {
		return err
	}

	ephkey := make([]byte, rovy.PublicKeySize)
	if err = binary.Read(r, binary.BigEndian, ephkey); err != nil {
		return err
	}
	pkt.Ephemeral = rovy.NewPublicKey(ephkey)

	if err = binary.Read(r, binary.BigEndian, &pkt.HelloHeader.Static); err != nil {
		return err
	}

	if err = binary.Read(r, binary.BigEndian, &pkt.HelloHeader.Timestamp); err != nil {
		return err
	}

	payloadSize, err := varint.ReadUvarint(r)
	if err != nil {
		return err
	}
	payloadBytes := make([]byte, payloadSize)
	if err = binary.Read(r, binary.BigEndian, payloadBytes); err != nil {
		return err
	}
	pkt.Payload = payloadBytes

	return nil
}

const ResponsePacketSize = 4 + 4 + 4 + ikpsk2.ResponseHeaderSize

type ResponsePacket struct {
	MsgType       uint32
	SenderIndex   uint32
	ReceiverIndex uint32
	ikpsk2.ResponseHeader
	Payload []byte
}

func (pkt *ResponsePacket) MarshalBinary() ([]byte, error) {
	payloadSize := varint.ToUvarint(uint64(binary.Size(pkt.Payload)))

	buf := make([]byte, ResponsePacketSize+len(payloadSize)+len(pkt.Payload))
	w := bytes.NewBuffer(buf[:0])

	if err := binary.Write(w, binary.LittleEndian, pkt.MsgType); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.SenderIndex); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.ReceiverIndex); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.ResponseHeader.Ephemeral); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.ResponseHeader.Empty); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, payloadSize); err != nil {
		return buf[:], err
	}
	if err := binary.Write(w, binary.BigEndian, pkt.Payload); err != nil {
		return buf[:], err
	}

	return buf[:], nil
}

func (pkt *ResponsePacket) UnmarshalBinary(buf []byte) (err error) {
	r := bytes.NewBuffer(buf)

	if err = binary.Read(r, binary.LittleEndian, &pkt.MsgType); err != nil {
		return err
	}

	if err = binary.Read(r, binary.BigEndian, &pkt.SenderIndex); err != nil {
		return err
	}

	if err = binary.Read(r, binary.BigEndian, &pkt.ReceiverIndex); err != nil {
		return err
	}

	ephkey := make([]byte, rovy.PublicKeySize)
	if err = binary.Read(r, binary.BigEndian, ephkey); err != nil {
		return err
	}
	pkt.Ephemeral = rovy.NewPublicKey(ephkey)

	if err = binary.Read(r, binary.BigEndian, &pkt.ResponseHeader.Empty); err != nil {
		return err
	}

	payloadSize, err := varint.ReadUvarint(r)
	if err != nil {
		return err
	}
	payloadBytes := make([]byte, payloadSize)
	if err = binary.Read(r, binary.BigEndian, payloadBytes); err != nil {
		return err
	}
	pkt.Payload = payloadBytes

	return nil
}

const DataPacketSize = 4 + 4 + ikpsk2.MessageHeaderSize

type DataPacket struct {
	MsgType       uint32
	ReceiverIndex uint32
	ikpsk2.MessageHeader
	Data []byte
}

func (pkt *DataPacket) MarshalBinary() ([]byte, error) {
	buf := make([]byte, DataPacketSize+len(pkt.Data))
	w := bytes.NewBuffer(buf[:0])

	if err := binary.Write(w, binary.LittleEndian, pkt.MsgType); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.ReceiverIndex); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.MessageHeader.Nonce); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.Data); err != nil {
		return buf[:], err
	}

	return buf[:], nil
}

func (pkt *DataPacket) UnmarshalBinary(buf []byte) (err error) {
	r := bytes.NewBuffer(buf)

	if err = binary.Read(r, binary.LittleEndian, &pkt.MsgType); err != nil {
		return err
	}

	if err = binary.Read(r, binary.BigEndian, &pkt.ReceiverIndex); err != nil {
		return err
	}

	if err = binary.Read(r, binary.BigEndian, &pkt.MessageHeader.Nonce); err != nil {
		return err
	}

	pkt.Data = make([]byte, len(buf)-DataPacketSize)
	if err = binary.Read(r, binary.BigEndian, &pkt.Data); err != nil {
		return err
	}

	return nil
}

const PlaintextPacketSize = 4 + SignatureSize + RandomizerSize + rovy.PublicKeySize

const SignatureSize = 8
const RandomizerSize = 8

// XXX who knows if this is makes any sense lol
type PlaintextPacket struct {
	MsgType   uint32
	Signature [SignatureSize]byte
	Random    [RandomizerSize]byte
	Sender    rovy.PeerID
	Data      []byte
}

func (pkt *PlaintextPacket) MarshalBinary() ([]byte, error) {
	pkt.MsgType = PlaintextMsgType

	buf := make([]byte, PlaintextPacketSize+len(pkt.Data))
	w := bytes.NewBuffer(buf[:0])

	if err := binary.Write(w, binary.LittleEndian, pkt.MsgType); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.Signature); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.Random); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.Sender); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.Data); err != nil {
		return buf[:], err
	}

	return buf[:], nil
}

func (pkt *PlaintextPacket) UnmarshalBinary(buf []byte) error {
	r := bytes.NewBuffer(buf)

	if err := binary.Read(r, binary.LittleEndian, &pkt.MsgType); err != nil {
		return err
	}

	if err := binary.Read(r, binary.BigEndian, &pkt.Signature); err != nil {
		return err
	}

	if err := binary.Read(r, binary.BigEndian, &pkt.Random); err != nil {
		return err
	}

	sender := make([]byte, rovy.PublicKeySize)
	if err := binary.Read(r, binary.BigEndian, sender); err != nil {
		return err
	}
	pkt.Sender = rovy.NewPeerID(rovy.NewPublicKey(sender))

	pkt.Data = make([]byte, len(buf)-PlaintextPacketSize)
	if err := binary.Read(r, binary.BigEndian, &pkt.Data); err != nil {
		return err
	}

	pkt.MsgType = PlaintextMsgType
	return nil
}
