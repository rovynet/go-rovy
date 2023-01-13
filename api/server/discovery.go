package rovyapis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	rovyapi "go.rovy.net/api"
)

func (s *Server) serveDiscoveryLinkLocalStart(w http.ResponseWriter, r *http.Request) {
	params := struct{ Interval string }{}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		s.writeError(w, r, fmt.Errorf("params: %s", err))
		return
	}

	interval, err := time.ParseDuration(params.Interval)
	if err != nil {
		s.writeError(w, r, fmt.Errorf("params: %s", err))
		return
	}

	opts := rovyapi.DiscoveryLinkLocal{Interval: interval}
	err = s.node.Discovery().StartLinkLocal(opts)
	if err != nil {
		s.writeError(w, r, fmt.Errorf("linklocal/start: %s", err))
		return
	}

	w.WriteHeader(http.StatusOK)
	s.logger.Printf("api request %s -> ok", r.RequestURI)
}

func (s *Server) serveDiscoveryLinkLocalStop(w http.ResponseWriter, r *http.Request) {
	if err := s.node.Discovery().StopLinkLocal(); err != nil {
		s.writeError(w, r, fmt.Errorf("linklocal/stop: %s", err))
		return
	}

	w.WriteHeader(http.StatusOK)
	s.logger.Printf("api request %s -> ok", r.RequestURI)
}
