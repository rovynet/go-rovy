package session

import (
	"bytes"
	"encoding/binary"

	varint "github.com/multiformats/go-varint"

	rovy "pkt.dev/go-rovy"
	ikpsk2 "pkt.dev/go-rovy/session/ikpsk2"
)

type HelloPacket struct {
	MsgType     uint32
	SenderIndex uint32
	ikpsk2.HelloHeader
	PeerID rovy.PeerID // TODO: remove once we can extract peerid from handshake
}

func (pkt *HelloPacket) MarshalBinary() ([]byte, error) {
	var buf [rovy.PreliminaryMTU]byte
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

	if err := rovy.PeerID2Buf(pkt.PeerID, w); err != nil {
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

	pkt.PeerID, err = rovy.Buf2PeerID(r)
	return err
}

type ResponsePacket struct {
	MsgType       uint32
	SenderIndex   uint32
	ReceiverIndex uint32
	ikpsk2.ResponseHeader
	PeerID rovy.PeerID // TODO: remove once we can extract peerid from handshake
}

func (pkt *ResponsePacket) MarshalBinary() ([]byte, error) {
	var buf [rovy.PreliminaryMTU]byte
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

	if err := rovy.PeerID2Buf(pkt.PeerID, w); err != nil {
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

	pkt.PeerID, err = rovy.Buf2PeerID(r)
	return err
}

// TODO: ikpsk2.DataHeader
type DataPacket struct {
	MsgType       uint32
	ReceiverIndex uint32
	Data          []byte
}

func (pkt *DataPacket) MarshalBinary() ([]byte, error) {
	var buf [rovy.PreliminaryMTU]byte
	w := bytes.NewBuffer(buf[:0])

	if err := binary.Write(w, binary.LittleEndian, pkt.MsgType); err != nil {
		return buf[:], err
	}

	if err := binary.Write(w, binary.BigEndian, pkt.ReceiverIndex); err != nil {
		return buf[:], err
	}

	dataSize := varint.ToUvarint(uint64(binary.Size(pkt.Data)))
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
