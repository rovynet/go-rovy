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

func (r *Routing) AddRoute(peerid rovy.PeerID, label rovy.Route) {
	r.Lock()
	defer r.Unlock()

	labels, present := r.table[peerid]
	if present {
		for _, l := range labels {
			if l.Equal(label) {
				return
			}
		}
		r.table[peerid] = append(labels, label)
	} else {
		r.table[peerid] = []rovy.Route{label}
	}
}

func (r *Routing) GetRoute(peerid rovy.PeerID) (rovy.Route, error) {
	r.RLock()
	defer r.RUnlock()

	labels, present := r.table[peerid]
	if !present || len(labels) == 0 {
		return nil, ErrUnknownPeerID
	}

	return labels[0], nil
}

func (r *Routing) MustGetRoute(peerid rovy.PeerID) rovy.Route {
	label, err := r.GetRoute(peerid)
	if err != nil {
		panic(err)
	}
	return label
}

func (r *Routing) PrintTable(out *log.Logger) {
	for peerid, labels := range r.table {
		out.Printf("/rovy/%s", peerid)
		for _, l := range labels {
			out.Printf("  /rovyfwd/%s", l)
		}
	}
}
