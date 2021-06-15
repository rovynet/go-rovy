package rovy

import (
	"bytes"
	"fmt"
	"strings"
)

const RouteSeparator = "."

type Route struct {
	hops []byte
}

func NewRoute(hops ...byte) Route {
	return Route{hops}
}

func (r Route) Bytes() []byte {
	return r.hops[:]
}

func (r Route) Len() int {
	return len(r.hops)
}

func (r Route) Empty() bool {
	return len(r.hops) == 0
}

func (r Route) Equal(other Route) bool {
	return bytes.Equal(r.Bytes(), other.Bytes())
}

func (r Route) Join(other Route) Route {
	return NewRoute(append(r.Bytes(), other.Bytes()...)...)
}

func (r Route) String() string {
	var str []string
	for _, b := range r.hops {
		str = append(str, fmt.Sprintf("%.2x", b))
	}
	return strings.Join(str, RouteSeparator)
}

func (r Route) Reverse() Route {
	var rev []byte
	for i := len(r.hops) - 1; i >= 0; i-- {
		rev = append(rev, r.hops[i])
	}
	return NewRoute(rev...)
}
