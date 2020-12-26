// Package forwarder implements a simple label-shifting packet switch.
//
// It provides the Forwarder type, which packet sending functions can be attached to.
// Each function gets assigned a slot number. For each incoming packet, the function
// corresponding to the packet route label's next hop will be called.
//
// All slot numbers of a forwarder must be of the same length (i.e. 1-byte, 2-byte, ...)
// so that route labels don't change in length while the packet passes through the forwarder.
// This is important for forwarding performance since it avoids realigning the packet buffer,
// but also helps with future Path MTU stuff. Rule of thumb: a forwarder's slot number length
// is known only to itself, not to other forwarders. Each forwarder knows how many bytes
// (or maybe even bits) to pop from and push to the route label, for everybody else that
// forwarder's part of the route label is opaque data. Each forwarder can pick its own
// number of slot numbers, for example based on the expected number of peerings.
//
// The libp2p-trained mind would point to varints to encode each hop's slot number as
// well as the total lenght of the route label, which would be pretty nice and convenient.
// However, there are a few good reason for a more compact approach. Instead of compressing
// individual numbers, we'll try to compress the whole, so the speak. [...]
//
// TODO: label should be bunch of varints (no, instead )
// TODO: length should be a varint (no, routes > 255 byte are absolutely fine for v0)

package forwarder

import (
	"errors"
	"fmt"
	"sync"

	rovy "pkt.dev/go-rovy"
)

const (
	NumSlots = 2 ^ 8
)

var (
	ErrPrevHopUnknown = errors.New("no slot for previous hop")
	ErrZeroLenLabel   = errors.New("got zero-length route label")
	ErrLabelTooLong   = errors.New("label is longer than 255 bytes")
	ErrLoopLabel      = errors.New("label results in loop")
)

type slotentry struct {
	peerid *rovy.PeerID
	send   sendFunc
}

type sendFunc func([]byte) error

func nullSendFunc(b []byte) error { return nil }

// TODO: is rovy.PeerID okay as a map index type?
type Forwarder struct {
	sync.RWMutex
	slots  map[int]slotentry
	bypeer map[rovy.PeerID]int
}

func NewForwarder() *Forwarder {
	fwd := &Forwarder{
		slots:  make(map[int]slotentry, NumSlots),
		bypeer: make(map[rovy.PeerID]int, NumSlots),
	}
	for i := 0; i < NumSlots; i++ {
		fwd.slots[i] = slotentry{send: nullSendFunc}
	}
	return fwd
}

func (fwd *Forwarder) Attach(peerid rovy.PeerID, send sendFunc) error {
	fwd.Lock()
	defer fwd.Unlock()

	for i := 0; i < NumSlots; i++ {
		slot := fwd.slots[i]
		if slot.peerid == nil || *slot.peerid == peerid {
			slot.peerid = &peerid
			slot.send = send
			fwd.bypeer[peerid] = i
			return nil
		}
	}
	return fmt.Errorf("no free forwarder slots")
}

func (fwd *Forwarder) Detach(peerid rovy.PeerID) {
	fwd.Lock()
	defer fwd.Unlock()

	for i := 0; i < NumSlots; i++ {
		slot := fwd.slots[i]
		if *slot.peerid == peerid {
			slot.peerid = nil
			slot.send = nullSendFunc
			delete(fwd.bypeer, peerid)
			return
		}
	}
}

// type stubdatapkt struct {
// 	pos   uint8
// 	len   uint8
// 	label [len]byte
// 	data  []byte
// }

func (fwd *Forwarder) HandlePacket(buf []byte, peerid rovy.PeerID) error {
	labellength := buf[1]
	if labellength == 0 {
		return ErrZeroLenLabel
	}

	labelpos := buf[0]
	if labelpos > labellength {
		return ErrLoopLabel
	}

	fwd.RLock()
	defer fwd.RUnlock()

	next := buf[2]
	prev, present := fwd.bypeer[peerid]
	if !present {
		return ErrPrevHopUnknown
	}

	// shift the label by 1 byte, popping the next hop (that's us)
	if labellength > 1 {
		// log.Printf("buf[3]=%+v buf[:]=%+v", buf[3:4], buf[:])
		copy(buf[2:], buf[3:labellength+1])
	}

	// now add the previous hop at the end, building up the reverse route
	buf[labellength+1] = byte(prev)

	// update the position counter
	buf[0] = labelpos + 1

	// and off the packet goes
	send := fwd.slots[int(next)].send
	return send(buf)
}

func (fwd *Forwarder) SendPacket(data []byte, peerid rovy.PeerID, label []byte) error {
	if len(label) == 0 {
		return ErrZeroLenLabel
	}
	if len(label) > 255 {
		return ErrLabelTooLong
	}

	buf := make([]byte, 2+len(label)+len(data))
	buf[0] = 0 // position counter
	buf[1] = byte(len(label))
	copy(buf[2:], label)
	copy(buf[2+buf[1]:], data)

	return fwd.HandlePacket(buf, peerid)
}

func (fwd *Forwarder) Overhead(label []byte) uint64 {
	return uint64(len(label) + 2)
}
