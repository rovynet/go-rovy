package bufpool

import (
	"fmt"
	"time"
)

// - Packet uses Buffer instead of []byte
// - Packet is released correctly everywhere

type BufferPool struct {
	fifo  chan *Buffer
	store []*Buffer
	time  time.Time
}

func NewBufferPool(poolsize, bufsize int) *BufferPool {
	pool := &BufferPool{
		fifo:  make(chan *Buffer, poolsize),
		store: make([]*Buffer, poolsize),
		time:  time.Now(),
	}
	for i := 0; i < poolsize; i++ {
		buf := NewBuffer(bufsize)
		buf.time = pool.time
		pool.store[i] = buf
		pool.fifo <- buf
	}
	go pool.leaksRoutine()
	return pool
}

// TODO: should be called from external only, don't spawn goroutine ourselves
func (pool *BufferPool) leaksRoutine() {
	sweepInterval := 1 * time.Second
	leakThreshold := 2 * time.Second

	for _ = range time.Tick(sweepInterval) {
		pool.time = time.Now()

		for _, buf := range pool.store {
			thrsh := buf.time.Add(leakThreshold)
			if buf.inuse && pool.time.After(thrsh) {
				panic(fmt.Sprintf("leaked buffer: pooltime=%s thrsh=%s buf=%+v", pool.time, thrsh, buf))
			}
		}
	}
}

func (pool *BufferPool) Release(buf *Buffer) {
	if !buf.inuse {
		panic("buffer has already been released")
	}
	buf.inuse = false
	pool.fifo <- buf
}

func (pool *BufferPool) Get() *Buffer {
	buf := <-pool.fifo
	buf.inuse = true
	buf.time = pool.time
	return buf
}

type Buffer struct {
	buf   []byte
	inuse bool
	time  time.Time
}

func NewBuffer(bufsize int) *Buffer {
	buf := &Buffer{
		buf:   make([]byte, bufsize),
		inuse: false,
	}
	return buf
}

func (buf *Buffer) Get() []byte {
	if !buf.inuse {
		panic("buffer has already been released")
	}
	return buf.buf
}
