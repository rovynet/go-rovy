package routing

import (
	"errors"
	"log"
	"sync"

	rovy "pkt.dev/go-rovy"
)

var (
	ErrUnknownPeerID = errors.New("no routes for this PeerID")
)

// TODO: is rovy.PeerID okay as a map index type?
type Routing struct {
	sync.RWMutex
	table  map[rovy.PeerID][]rovy.Route
	logger *log.Logger
}

func NewRouting(logger *log.Logger) *Routing {
	return &Routing{
		table:  make(map[rovy.PeerID][]rovy.Route),
		logger: logger,
	}
}

func (r *Routing) AddRoute(peerid rovy.PeerID, route rovy.Route) {
	r.Lock()
	defer r.Unlock()

	routes, present := r.table[peerid]
	if present {
		for _, l := range routes {
			if l.Equal(route) {
				return
			}
		}
		r.table[peerid] = append(routes, route)
	} else {
		r.table[peerid] = []rovy.Route{route}
	}
}

func (r *Routing) GetRoute(peerid rovy.PeerID) (rovy.Route, error) {
	r.RLock()
	defer r.RUnlock()

	routes, present := r.table[peerid]
	if !present || len(routes) == 0 {
		return nil, ErrUnknownPeerID
	}

	return routes[0], nil
}

func (r *Routing) MustGetRoute(peerid rovy.PeerID) rovy.Route {
	route, err := r.GetRoute(peerid)
	if err != nil {
		panic(err)
	}
	return route
}

func (r *Routing) PrintTable(out *log.Logger) {
	for peerid, routes := range r.table {
		out.Printf("/rovy/%s", peerid)
		for _, l := range routes {
			out.Printf("  /rovyrt/%s", l)
		}
	}
}
