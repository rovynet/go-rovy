package session

import (
	"bytes"
	"encoding/binary"
	"fmt"

	cid "github.com/ipfs/go-cid" // TOOD: kill this dep
	multiaddr "github.com/multiformats/go-multiaddr"
	varint "github.com/multiformats/go-varint"
	rovy "pkt.dev/go-rovy"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/poly1305"
	"golang.zx2c4.com/wireguard/tai64n"
)

const (
	MaxMultiaddrSize = 128
	MaxPeerIDSize    = 128
)

// TODO: the endianess is off everywhere in this file

type HelloPacket struct {
	HelloHeader
	ObservedMTU  uint64
	ObservedAddr multiaddr.Multiaddr
	PeerID       rovy.PeerID
}

type HelloHeader struct {
	MsgType     uint8
	Reserved    [3]uint8
	SenderIndex uint32
	Ephemeral   rovy.PublicKey
	Static      [rovy.PublicKeySize + poly1305.TagSize]byte
	Timestamp   [tai64n.TimestampSize + poly1305.TagSize]byte
	MAC1        [blake2s.Size128]byte
	MAC2        [blake2s.Size128]byte
}

func (pkt *HelloPacket) MarshalBinary() ([]byte, error) {
	var buf [rovy.PreliminaryMTU]byte
	w := bytes.NewBuffer(buf[:0])

	if err := binary.Write(w, binary.LittleEndian, pkt.HelloHeader); err != nil {
		return buf[:], err
	}

	mtu := varint.ToUvarint(pkt.ObservedMTU)
	if err := binary.Write(w, binary.LittleEndian, mtu); err != nil {
		return buf[:], err
	}

	maddrBytes := pkt.ObservedAddr.Bytes()
	maddrSize := varint.ToUvarint(uint64(binary.Size(maddrBytes)))
	if err := binary.Write(w, binary.LittleEndian, maddrSize); err != nil {
		return buf[:], err
	}
	if err := binary.Write(w, binary.LittleEndian, maddrBytes); err != nil {
		return buf[:], err
	}

	peeridBytes := pkt.PeerID.Bytes()
	peeridSize := varint.ToUvarint(uint64(binary.Size(peeridBytes)))
	if err := binary.Write(w, binary.LittleEndian, peeridSize); err != nil {
		return buf[:], err
	}
	if err := binary.Write(w, binary.LittleEndian, peeridBytes); err != nil {
		return buf[:], err
	}

	return buf[:], nil
}

func (pkt *HelloPacket) UnmarshalBinary(buf []byte) (err error) {
	if len(buf) > rovy.PreliminaryMTU {
		buf = buf[:rovy.PreliminaryMTU]
	}
	r := bytes.NewBuffer(buf)

	if err = binary.Read(r, binary.LittleEndian, &pkt.HelloHeader); err != nil {
		return err
	}

	pkt.ObservedMTU, err = varint.ReadUvarint(r)
	if err != nil {
		return err
	}

	maddrSize, err := varint.ReadUvarint(r)
	if err != nil {
		return err
	}
	if maddrSize > MaxMultiaddrSize {
		return fmt.Errorf("multiaddr too long")
	}
	maddrBytes := make([]byte, maddrSize)
	if err = binary.Read(r, binary.LittleEndian, maddrBytes); err != nil {
		return err
	}
	pkt.ObservedAddr, err = multiaddr.NewMultiaddrBytes(maddrBytes[:maddrSize])
	if err != nil {
		return err
	}

	peeridSize, err := varint.ReadUvarint(r)
	if err != nil {
		return err
	}
	if peeridSize > MaxPeerIDSize {
		return fmt.Errorf("PeerID too long")
	}
	peeridBytes := make([]byte, peeridSize)
	if err = binary.Read(r, binary.LittleEndian, peeridBytes); err != nil {
		return err
	}
	_, c, err := cid.CidFromBytes(peeridBytes[:peeridSize]) // TODO this should be on PeerID itself
	if err != nil {
		return err
	}
	pkt.PeerID, err = rovy.NewPeerIDFromCid(c)
	if err != nil {
		return err
	}

	return nil
}

type HelloResponsePacket struct {
	HelloResponseHeader
	ObservedMTU  uint64
	ObservedAddr multiaddr.Multiaddr
	PeerID       rovy.PeerID
}

type HelloResponseHeader struct {
	MsgType       uint8
	Reserved      [3]uint8
	SenderIndex   uint32
	ReceiverIndex uint32
	Ephemeral     rovy.PublicKey
	Static        [rovy.PublicKeySize + poly1305.TagSize]byte
	Timestamp     [tai64n.TimestampSize + poly1305.TagSize]byte
	MAC1          [blake2s.Size128]byte
	MAC2          [blake2s.Size128]byte
}

func (pkt *HelloResponsePacket) MarshalBinary() ([]byte, error) {
	var buf [rovy.PreliminaryMTU]byte
	w := bytes.NewBuffer(buf[:0])

	if err := binary.Write(w, binary.LittleEndian, pkt.HelloResponseHeader); err != nil {
		return buf[:], err
	}

	mtu := varint.ToUvarint(pkt.ObservedMTU)
	if err := binary.Write(w, binary.LittleEndian, mtu); err != nil {
		return buf[:], err
	}

	maddrBytes := pkt.ObservedAddr.Bytes()
	maddrSize := varint.ToUvarint(uint64(binary.Size(maddrBytes)))
	if err := binary.Write(w, binary.LittleEndian, maddrSize); err != nil {
		return buf[:], err
	}
	if err := binary.Write(w, binary.LittleEndian, maddrBytes); err != nil {
		return buf[:], err
	}

	peeridBytes := pkt.PeerID.Bytes()
	peeridSize := varint.ToUvarint(uint64(binary.Size(peeridBytes)))
	if err := binary.Write(w, binary.LittleEndian, peeridSize); err != nil {
		return buf[:], err
	}
	if err := binary.Write(w, binary.LittleEndian, peeridBytes); err != nil {
		return buf[:], err
	}

	return buf[:], nil
}

func (pkt *HelloResponsePacket) UnmarshalBinary(buf []byte) (err error) {
	if len(buf) > rovy.PreliminaryMTU {
		buf = buf[:rovy.PreliminaryMTU]
	}
	r := bytes.NewBuffer(buf)

	if err = binary.Read(r, binary.LittleEndian, &pkt.HelloResponseHeader); err != nil {
		return err
	}

	pkt.ObservedMTU, err = varint.ReadUvarint(r)
	if err != nil {
		return err
	}

	maddrSize, err := varint.ReadUvarint(r)
	if err != nil {
		return err
	}
	if maddrSize > MaxMultiaddrSize {
		return fmt.Errorf("multiaddr too long")
	}
	maddrBytes := make([]byte, maddrSize)
	if err = binary.Read(r, binary.LittleEndian, maddrBytes); err != nil {
		return err
	}
	pkt.ObservedAddr, err = multiaddr.NewMultiaddrBytes(maddrBytes[:maddrSize])
	if err != nil {
		return err
	}

	peeridSize, err := varint.ReadUvarint(r)
	if err != nil {
		return err
	}
	if peeridSize > MaxPeerIDSize {
		return fmt.Errorf("PeerID too long")
	}
	peeridBytes := make([]byte, peeridSize)
	if err = binary.Read(r, binary.LittleEndian, peeridBytes); err != nil {
		return err
	}
	_, c, err := cid.CidFromBytes(peeridBytes[:peeridSize]) // TODO this should be on PeerID itself
	if err != nil {
		return err
	}
	pkt.PeerID, err = rovy.NewPeerIDFromCid(c)
	if err != nil {
		return err
	}

	return nil
}

type DataPacket struct {
	DataHeader
	Data []byte
}

type DataHeader struct {
	MsgType       uint8
	Reserved      [3]uint8
	ReceiverIndex uint32
}

func (pkt *DataPacket) MarshalBinary() ([]byte, error) {
	var buf [rovy.PreliminaryMTU]byte
	w := bytes.NewBuffer(buf[:0])

	if err := binary.Write(w, binary.LittleEndian, pkt.DataHeader); err != nil {
		return buf[:], err
	}

	dataSize := varint.ToUvarint(uint64(binary.Size(pkt.Data)))
	if err := binary.Write(w, binary.LittleEndian, dataSize); err != nil {
		return buf[:], err
	}
	if err := binary.Write(w, binary.LittleEndian, pkt.Data); err != nil {
		return buf[:], err
	}

	return buf[:], nil
}

func (pkt *DataPacket) UnmarshalBinary(buf []byte) (err error) {
	if len(buf) > rovy.PreliminaryMTU {
		buf = buf[:rovy.PreliminaryMTU]
	}
	r := bytes.NewBuffer(buf)

	if err = binary.Read(r, binary.LittleEndian, &pkt.DataHeader); err != nil {
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
	if err = binary.Read(r, binary.LittleEndian, dataBytes); err != nil {
		return err
	}
	pkt.Data = dataBytes

	return nil
}
