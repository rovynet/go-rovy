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
	"log"
	"sync"

	rovy "pkt.dev/go-rovy"
)

var (
	ErrPrevHopUnknown = errors.New("no slot for previous hop")
	ErrNextHopUnknown = errors.New("no slot for next hop")
	ErrZeroLenLabel   = errors.New("got zero-length route label")
	ErrLabelTooLong   = errors.New("label is longer than 255 bytes")
	ErrLoopLabel      = errors.New("label resulted in loop")

	nullSlotEntry = &slotentry{
		rovy.NullPeerID,
		func(_ rovy.PeerID, _ []byte) error { return nil },
	}
)

type slotentry struct {
	peerid rovy.PeerID
	send   sendFunc
}

type sendFunc func(rovy.PeerID, []byte) error

// TODO: is rovy.PeerID okay as a map index type?
type Forwarder struct {
	sync.RWMutex
	size   int
	slots  map[int]*slotentry
	bypeer map[rovy.PeerID]int
	logger *log.Logger
}

func NewForwarder(size int, logger *log.Logger) *Forwarder {
	fwd := &Forwarder{
		size:   size,
		slots:  make(map[int]*slotentry, size),
		bypeer: make(map[rovy.PeerID]int, size),
		logger: logger,
	}
	for i := 0; i < fwd.size; i++ {
		fwd.slots[i] = nullSlotEntry
	}
	return fwd
}

func (fwd *Forwarder) Attach(peerid rovy.PeerID, send sendFunc) (rovy.Route, error) {
	fwd.Lock()
	defer fwd.Unlock()

	for i := 0; i < fwd.size; i++ {
		if fwd.slots[i] == nullSlotEntry {
			fwd.slots[i] = &slotentry{peerid, send}
			fwd.bypeer[peerid] = i
			return rovy.Route([]byte{byte(i)}), nil
		}
	}
	return nil, fmt.Errorf("no free slots")
}

func (fwd *Forwarder) Detach(peerid rovy.PeerID) error {
	fwd.Lock()
	defer fwd.Unlock()

	for i := 0; i < fwd.size; i++ {
		if fwd.slots[i].peerid.Equal(peerid) {
			fwd.slots[i] = nullSlotEntry
			delete(fwd.bypeer, peerid)
			return nil
		}
	}
	return fmt.Errorf("slot entry not found")
}

// type stubdatapkt struct {
// 	pos   uint8
// 	len   uint8
// 	label [len]byte
// 	data  []byte
// }

func (fwd *Forwarder) HandlePacket(buf []byte, from rovy.PeerID) error {
	// fwd.logger.Printf("fwd: %#v", fwd)
	// fwd.logger.Printf("in: %#v", buf)

	length := buf[1]
	if length == 0 {
		return ErrZeroLenLabel
	}

	pos := buf[0]
	if pos > length {
		return ErrLoopLabel
	}

	fwd.RLock()
	defer fwd.RUnlock()

	prev, present := fwd.bypeer[from]
	if !present {
		return ErrPrevHopUnknown
	}

	next := buf[2+pos]
	buf[2+pos] = byte(prev)
	buf[0] = pos + 1

	// fwd.logger.Printf("out: %#v", buf)

	// and off the packet goes
	return fwd.slots[int(next)].send(from, buf)
}

func (fwd *Forwarder) SendPacket(data []byte, from rovy.PeerID, label rovy.Route) error {
	length := len(label)

	if length == 0 {
		return ErrZeroLenLabel
	}
	if length > 255 {
		return ErrLabelTooLong
	}

	buf := make([]byte, 2+length+len(data))
	buf[0] = 0 // position counter
	buf[1] = byte(length)
	copy(buf[2:], label)
	copy(buf[2+length:], data)

	return fwd.HandlePacket(buf, from)
}

func (fwd *Forwarder) StripHeader(buf []byte) []byte {
	return buf[2+buf[1]:]
}
