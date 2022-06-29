package rovyapis

import (
	"encoding/json"
	"fmt"
	"net/http"

	rovy "go.rovy.net"
)

func (s *Server) servePeerStatus(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
}

func (s *Server) servePeerListen(w http.ResponseWriter, r *http.Request) {
	params := struct{ Addr rovy.Multiaddr }{}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		s.writeError(w, r, fmt.Errorf("params: %s", err))
		return
	}

	pl, err := s.node.Peer().Listen(params.Addr)
	if err != nil {
		s.writeError(w, r, fmt.Errorf("peer.enable: %s", err))
		return
	}

	out, err := json.Marshal(&pl)
	if err != nil {
		s.writeError(w, r, fmt.Errorf("json: %s", err))
		return
	}
	w.WriteHeader(http.StatusOK)
	out = append(out, 0x0a) // newline
	_, _ = w.Write(out)

	s.logger.Printf("api request %s -> ok", r.RequestURI)
}

func (s *Server) servePeerConnect(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
}

func (s *Server) servePeerPolicy(w http.ResponseWriter, r *http.Request) {
	var params []string
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		s.writeError(w, r, fmt.Errorf("params: %s", err))
		return
	}

	err := s.node.Peer().Policy(params...)
	if err != nil {
		s.writeError(w, r, fmt.Errorf("peer/policy: %s", err))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("{}\n"))

	s.logger.Printf("api request %s -> ok", r.RequestURI)
}
