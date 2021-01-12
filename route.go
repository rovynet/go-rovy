package rovy

import (
	"bytes"
)

type Route []byte

func (r Route) Bytes() []byte {
	return []byte(r)
}

func (r Route) Equal(other Route) bool {
	return bytes.Equal(r.Bytes(), other.Bytes())
}

func (r Route) Join(other Route) Route {
	return Route(append(r, other...))
}
