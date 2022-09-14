package bufpool

type buffer struct {
	id int
	b  []byte
}

type Pool struct {
	ring  chan buffer    // holds available buffers
	table map[int]buffer // buffers that are in use, for introspection
	zero  []byte         // for zero'ing on release
}

func NewPool(poolsize, bufsize int) *Pool {
	pool := &Pool{
		ring:  make(chan buffer, poolsize),
		table: make(map[int]buffer, poolsize),
		zero:  make([]byte, bufsize),
	}
	for id := 1; id <= poolsize; id++ {
		pool.ring <- buffer{id, make([]byte, bufsize)}
	}
	for i, _ := range pool.zero {
		pool.zero[i] = 0
	}
	return pool
}

func (pool *Pool) Get() ([]byte, int) {
	if len(pool.ring) == 0 {
		panic("no buffers available")
	}
	buf := <-pool.ring
	// pool.table[buf.id] = buf
	return buf.b, buf.id
}

func (pool *Pool) Release(b []byte, id int) {
	// delete(pool.table, id)
	copy(b[:], pool.zero)
	pool.ring <- buffer{id, b}
}
