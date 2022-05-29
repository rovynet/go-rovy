package ringbuf

import (
	rovy "go.rovy.net"
)

type RingBuffer struct {
	ch      chan rovy.Packet
	dropped uint64
}

func NewRingBuffer(capacity int) *RingBuffer {
	rb := &RingBuffer{ch: make(chan rovy.Packet, capacity)}
	return rb
}

func (rb *RingBuffer) Put(pkt rovy.Packet) {
	select {
	case rb.ch <- pkt:
		// ok
	default:
		<-rb.ch // drop oldest packet to make space
		rb.dropped += 1
		rb.ch <- pkt
	}
}

func (rb *RingBuffer) PutWithBackpressure(pkt rovy.Packet) {
	rb.ch <- pkt
}

// XXX returns empty packet if chan is closed
func (rb *RingBuffer) Get() rovy.Packet {
	return <-rb.ch
}

func (rb *RingBuffer) Chan() chan rovy.Packet {
	return rb.ch
}

func (rb *RingBuffer) Capacity() int {
	return cap(rb.ch)
}

func (rb *RingBuffer) Length() int {
	return len(rb.ch)
}

func (rb *RingBuffer) Close() {
	for len(rb.ch) > 0 {
		_ = <-rb.ch
	}
	close(rb.ch)
}

func (rb *RingBuffer) Dropped() uint64 {
	return rb.dropped
}
