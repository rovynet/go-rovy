// Package forwarder implements a simple packet switch with route labels on every packet.
//
// Packet forwarding in Rovy puts a route label on every packet.
// This label describes the route the packet should traverse.
// At any hop this label also describes the reverse route,
// so that reply or error packets can be sent without any further lookups.
//
// ```
// [codec][len][pos][route][data...]
// ```
//
// - `codec` is the multicodec header for forwarder data packets,
//   usually negotiated using multigram during session establishment.
//   This is how the receiving end knows that they're dealing with a forwarder packet.
//
// - `len` is the byte length of `route`.
//   This is only one byte, so `route`'s length is limited to 255 bytes.
//   Also, one route byte equals one hop, because for now every forwarder has only 256 slots.
//   The are plans for how this limit can be elegantly lifted.
//
// - `pos` is the byte position within `route`
//   representing the position of the receiving forwarder on the route.
//   For the sending forwarder, the byte at `pos` is their slot number for the next hop,
//   and for the receiving forwarder, the byte at `pos - 1` is what
//   they'll overwrite with their own slot number for the previous hop.
//
// - `route` is the bytes representing the route, the length being determined by `len`.
//   The receiving forwarder takes the byte at `route[pos]`, treating it as the next hop,
//   and sends the packet to the peer identified by that slot number.
//   Before sending the packet, the `pos` counter is incremented by one.
//   If `pos` is equal to `len`, the receiving forwarder is the last hop,
//   and consume the received packet into its upper layer.
//   One way or the other, the byte at `route[pos - 1]` is replaced
//   by the slot number for the peer which the packet was received from.
//   That way we have the reverse route handy at every hop.
//
// - `data` is the actual payload of the packet, usually expected to be a Rovy session packet.
//
// All slot numbers of a forwarder must be of the same length (i.e. 1-byte, 2-byte, ...)
// so that route labels don't change in length while the packet passes through the forwarder.
// This is important for forwarding performance since it avoids realigning the packet buffer,
// but also helps with future Path MTU stuff. Rule of thumb: a forwarder's slot number length
// is known only to itself, not to other forwarders. Each forwarder can pick its own
// number of slot numbers, for example based on the expected number of peerings.
// Nevertheless towards other nodes, it needs to act as if each slot number was 1 byte,
// so it must for example increase the `pos` by 2 if it has 256^2 slots.
//
// Q: Why is "self" not represented in the route and the route rotated by one byte at every hop?
// A: We trade for a nicer human-readable representation of the route here.
//    We don't even save a byte because without the rotation and "self" as the end-marker,
//    we instead need to track the position in addition to the route.
//    It also means we can't just hand outgoing packets to `HandlePacket` because
//    it would add "self" as the previous hop.
//
// TODO: send error reply packets

package forwarder

import (
	"errors"
	"fmt"
	"log"
	"sync"

	varint "github.com/multiformats/go-varint"
	rovy "pkt.dev/go-rovy"
	multigram "pkt.dev/go-rovy/multigram"
)

const (
	NumSlots  = 256
	HopLength = 1

	DataMulticodec  = 0x12345
	ErrorMulticodec = 0x12346
)

var (
	ErrPrevHopUnknown = errors.New("no slot for previous hop")
	ErrNextHopUnknown = errors.New("no slot for next hop")
	ErrSelfUnknown    = errors.New("no slot for ourselves")
	ErrZeroLenRoute   = errors.New("got zero-length route route")
	ErrRouteTooLong   = errors.New("route is longer than 255 bytes")
	ErrLoopRoute      = errors.New("route resulted in loop")

	// TODO: move into NewForwarder so we can use logger
	nullSlotEntry = &slotentry{
		rovy.EmptyPeerID,
		func(from rovy.PeerID, _ []byte) error {
			fmt.Printf("nullSlotEntry: dropping packet for unknown destination from %s\n", from)
			return nil
		},
	}
)

type slotentry struct {
	peerid rovy.PeerID
	send   sendFunc
}

type sendFunc func(rovy.PeerID, []byte) error

// XXX: is rovy.PeerID okay as a map index type?
type Forwarder struct {
	sync.RWMutex
	slots  map[int]*slotentry
	bypeer map[rovy.PeerID]int
	mgram  *multigram.Table
	logger *log.Logger
}

func NewForwarder(mgram *multigram.Table, logger *log.Logger) *Forwarder {
	mgram.AddCodec(DataMulticodec)
	mgram.AddCodec(ErrorMulticodec)

	fwd := &Forwarder{
		slots:  make(map[int]*slotentry, NumSlots),
		bypeer: make(map[rovy.PeerID]int, NumSlots),
		mgram:  mgram,
		logger: logger,
	}
	for i := 0; i < NumSlots; i++ {
		fwd.slots[i] = nullSlotEntry
	}
	return fwd
}

func (fwd *Forwarder) PrintSlots(logger *log.Logger) {
	fwd.RLock()
	defer fwd.RUnlock()

	for i, se := range fwd.slots {
		if se.peerid != nullSlotEntry.peerid {
			logger.Printf("fwd: slot /rovyrt/%.2x => /rovy/%s", i, se.peerid)
		}
	}
}

func (fwd *Forwarder) Attach(peerid rovy.PeerID, send sendFunc) (rovy.Route, error) {
	fwd.Lock()
	defer fwd.Unlock()

	for i := 0; i < NumSlots; i++ {
		if fwd.slots[i] == nullSlotEntry {
			fwd.slots[i] = &slotentry{peerid, send}
			fwd.bypeer[peerid] = i
			return rovy.NewRoute(byte(i)), nil
		}
	}
	return rovy.NewRoute(), fmt.Errorf("no free slots")
}

func (fwd *Forwarder) Detach(peerid rovy.PeerID) error {
	fwd.Lock()
	defer fwd.Unlock()

	for i := 0; i < NumSlots; i++ {
		if fwd.slots[i].peerid.Equal(peerid) {
			fwd.slots[i] = nullSlotEntry
			delete(fwd.bypeer, peerid)
			return nil
		}
	}
	return fmt.Errorf("slot entry not found")
}

func (fwd *Forwarder) HandleError(buf []byte, from rovy.PeerID) error {
	code, _, err := varint.FromUvarint(buf)
	if err != nil {
		fwd.logger.Printf("got broken error reply from %s: %s", from, err)
		return nil
	}

	switch code {
	case 1:
		fwd.logger.Printf("error reply from %s: unknown slot", from)
	default:
		fwd.logger.Printf("error reply from %s: %d", from, code)
	}

	return nil
}

// TODO drop if n+2+length > len(buf) || n+2+pos > len(buf)+2
func (fwd *Forwarder) HandlePacket(buf []byte, from rovy.PeerID) error {
	length := int(buf[5])
	if length == 0 {
		return ErrZeroLenRoute
	}

	pos := int(buf[4])
	if pos > length {
		return ErrLoopRoute
	}

	fwd.RLock()
	defer fwd.RUnlock()

	next := buf[6+pos+1]

	prev, present := fwd.bypeer[from]
	if !present {
		return ErrPrevHopUnknown
	}
	buf[6+pos] = byte(prev)

	if pos+1 == length {
		return fwd.slots[0].send(from, buf)
	}
	buf[4] = byte(pos + 1)

	// fwd.logger.Printf("forwarder: packet from %s forwarded along %s", from, rovy.NewRoute(buf[6+pos:6+buf[5]]...))
	return fwd.slots[int(next)].send(from, buf)
}

// data is expected to have room for 20 bytes forwarder header at the beginning
func (fwd *Forwarder) SendPacket(data []byte, from rovy.PeerID, route rovy.Route) error {
	length := route.Len()

	if length == 0 {
		return ErrZeroLenRoute
	}
	if length > 14 {
		return ErrRouteTooLong
	}

	copy(data[0:2], fwd.mgram.ToUvarint(DataMulticodec))
	data[4] = 0x0 // pos
	data[5] = byte(length)
	copy(data[6:6+length], route.Bytes())
	for i := 6 + length; i < 20; i++ {
		data[i] = 0x0
	}

	// fwd.logger.Printf("forwarder: packet from us forwarded along %s", route)
	return fwd.slots[int(data[6])].send(from, data)
}
