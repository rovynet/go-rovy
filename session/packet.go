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

	if err := binary.Write(w, binary.BigEndian, pkt.HelloHeader); err != nil {
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
	if len(buf) > rovy.PreliminaryMTU {
		buf = buf[:rovy.PreliminaryMTU]
	}
	r := bytes.NewBuffer(buf)

	if err = binary.Read(r, binary.LittleEndian, &pkt.MsgType); err != nil {
		return err
	}

	if err = binary.Read(r, binary.BigEndian, &pkt.SenderIndex); err != nil {
		return err
	}

	if err = binary.Read(r, binary.BigEndian, &pkt.HelloHeader); err != nil {
		return err
	}

	payloadSize, err := varint.ReadUvarint(r)
	if err != nil {
		return err
	}
	if payloadSize > rovy.PreliminaryMTU-8 {
		payloadSize = rovy.PreliminaryMTU - 8
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

	if err := binary.Write(w, binary.BigEndian, pkt.ResponseHeader); err != nil {
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
	if len(buf) > rovy.PreliminaryMTU {
		buf = buf[:rovy.PreliminaryMTU]
	}
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

	if err = binary.Read(r, binary.BigEndian, &pkt.ResponseHeader); err != nil {
		return err
	}

	payloadSize, err := varint.ReadUvarint(r)
	if err != nil {
		return err
	}
	if payloadSize > rovy.PreliminaryMTU-8 {
		payloadSize = rovy.PreliminaryMTU - 8
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
	dataSize := varint.ToUvarint(uint64(binary.Size(pkt.Data)))

	buf := make([]byte, DataPacketSize+len(dataSize)+len(pkt.Data))
	w := bytes.NewBuffer(buf[:0])

	if err := binary.Write(w, binary.LittleEndian, pkt.MsgType); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.ReceiverIndex); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.MessageHeader); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, dataSize); err != nil {
		return buf[:], err
	}
	if err := binary.Write(w, binary.BigEndian, pkt.Data); err != nil {
		return buf[:], err
	}

	return buf[:], nil
}

func (pkt *DataPacket) UnmarshalBinary(buf []byte) (err error) {
	if len(buf) > rovy.PreliminaryMTU {
		buf = buf[:rovy.PreliminaryMTU]
	}
	r := bytes.NewBuffer(buf)

	if err = binary.Read(r, binary.LittleEndian, &pkt.MsgType); err != nil {
		return err
	}

	if err = binary.Read(r, binary.BigEndian, &pkt.ReceiverIndex); err != nil {
		return err
	}

	if err = binary.Read(r, binary.BigEndian, &pkt.MessageHeader); err != nil {
		return err
	}

	dataSize, err := varint.ReadUvarint(r)
	if err != nil {
		return err
	}
	if dataSize > rovy.PreliminaryMTU-8 {
		dataSize = rovy.PreliminaryMTU - 8
	}
	dataBytes := make([]byte, dataSize)
	if err = binary.Read(r, binary.BigEndian, dataBytes); err != nil {
		return err
	}
	pkt.Data = dataBytes

	return nil
}
