package rovy

import (
	"bytes"
	"fmt"
	"strings"
)

const RouteSeparator = "."

type Route []byte

func (r Route) Bytes() []byte {
	return []byte(r)
}

func (r Route) Len() int {
	return len(r)
}

func (r Route) Equal(other Route) bool {
	return bytes.Equal(r.Bytes(), other.Bytes())
}

func (r Route) Join(other Route) Route {
	return Route(append(r, other...))
}

func (r Route) String() string {
	var str []string
	for _, b := range r {
		str = append(str, fmt.Sprintf("%.2x", b))
	}
	return strings.Join(str, RouteSeparator)
}

func (r Route) Reverse() Route {
	var rev []byte
	for i := len(r) - 1; i >= 0; i-- {
		rev = append(rev, r[i])
	}
	return rev
}
