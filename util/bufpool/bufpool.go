package bufpool

import (
	// "fmt"
	"log"
	"runtime"
	"sync"
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
		buf.InUse = false
		pool.store[i] = buf
		pool.fifo <- buf
	}
	return pool
}

func (pool *BufferPool) LeaksRoutine() {
	sweepInterval := 2 * time.Second
	leakThreshold := 2 * time.Second

	for now := range time.Tick(sweepInterval) {
		pool.time = now

		for i, buf := range pool.store {
			// buf.RLock()
			thrsh := buf.time.Add(leakThreshold)
			if buf.InUse && now.After(thrsh) {
				// panic(fmt.Sprintf("leaked buffer: i=%d pooltime=%s thrsh=%s buftime=%s buf=%v", i, pool.time, thrsh, buf.time, buf))
				log.Printf("leaked buffer: i=%d pooltime=%s thrsh=%s buftime=%s buf=%v", i, pool.time, thrsh, buf.time, buf)
			}
			// buf.RUnlock()
		}
	}
}

func (pool *BufferPool) Release(buf *Buffer) {
	// buf.Lock()
	// defer buf.Unlock()
	if !buf.InUse {
		panic("buffer has already been released")
	}
	buf.InUse = false
	buf.callers = []uintptr{}
	pool.fifo <- buf
}

func (pool *BufferPool) Get() *Buffer {
	buf := <-pool.fifo
	if buf.InUse {
		panic("buffer has not been properly released")
	}
	// buf.Lock()
	// defer buf.Unlock()
	buf.time = pool.time
	buf.InUse = true
	_ = runtime.Callers(0, buf.callers)
	return buf
}

type Buffer struct {
	sync.RWMutex
	Buf     []byte
	InUse   bool
	time    time.Time
	callers []uintptr
}

func NewBuffer(bufsize int) *Buffer {
	buf := &Buffer{
		Buf: make([]byte, bufsize),
	}
	return buf
}

func (buf *Buffer) Get() []byte {
	// buf.RLock()
	// defer buf.RUnlock()
	if !buf.InUse {
		panic("buffer has already been released")
	}
	return buf.Buf
}
