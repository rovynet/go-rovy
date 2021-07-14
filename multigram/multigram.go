package multigram

import (
	"log"
	"sync"

	varint "github.com/multiformats/go-varint"
)

type Table struct {
	sync.RWMutex
	c2n  map[uint64]uint64
	n2c  map[uint64]uint64
	next uint64
}

func NewTable() *Table {
	return &Table{
		c2n:  map[uint64]uint64{},
		n2c:  map[uint64]uint64{},
		next: uint64(1),
	}
}

func NewTableFromBytes(buf []byte) (*Table, error) {
	t := NewTable()

	i := 0
	for {
		if i+2 > len(buf) {
			break
		}

		number, n, err := varint.FromUvarint(buf[i:])
		if err != nil {
			return t, err
		}
		i += n

		code, n, err := varint.FromUvarint(buf[i:])
		if err != nil {
			return t, err
		}
		i += n

		t.c2n[code] = number
		t.n2c[number] = code
		if number >= t.next {
			t.next = number + 1
		}
	}

	return t, nil
}

// XXX not sure if this is needed at all
func (t *Table) Clone() *Table {
	t.RLock()
	defer t.RUnlock()

	t2 := &Table{
		c2n:  map[uint64]uint64{},
		n2c:  map[uint64]uint64{},
		next: t.next,
	}
	for c, n := range t.c2n {
		t2.c2n[c] = n
		t2.n2c[n] = c
	}
	return t2
}

func (t *Table) Bytes() []byte {
	var buf []byte
	for code, number := range t.c2n {
		buf = append(buf, varint.ToUvarint(number)...)
		buf = append(buf, varint.ToUvarint(code)...)
	}
	return buf
}

func (t *Table) AddCodec(code uint64) uint64 {
	t.Lock()
	defer t.Unlock()

	number, present := t.c2n[code]
	if present {
		return number
	}

	number = t.next
	t.n2c[number] = code
	t.c2n[code] = t.next

	t.next += 1
	return number
}

func (t *Table) LookupCodec(number uint64) uint64 {
	t.RLock()
	defer t.RUnlock()

	code, present := t.n2c[number]
	if !present {
		return uint64(0)
	}
	return code
}

func (t *Table) LookupNumber(code uint64) uint64 {
	t.RLock()
	defer t.RUnlock()

	number, present := t.c2n[code]
	if !present {
		return uint64(0)
	}
	return number
}

func (t *Table) FromUvarint(buf []byte) (uint64, int, error) {
	number, n, err := varint.FromUvarint(buf)
	if err != nil {
		return uint64(0), 0, err
	}

	c := t.LookupCodec(number)
	if int(c) == 0 {
		log.Panicf("FromUvarint: c=%d n=%d buf=%#v tbl=%#v", c, number, buf, t.n2c)
	}

	return c, n, nil
}

func (t *Table) ToUvarint(code uint64) []byte {
	number := t.LookupNumber(code)
	if number == uint64(0) {
		log.Panicf("unknown multigram codec %d (0x%x)", code, code)
	}

	return varint.ToUvarint(number)
}
