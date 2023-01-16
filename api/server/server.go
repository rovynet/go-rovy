package rovyapis

import (
	"encoding/json"
	"log"
	"net"
	"net/http"

	mux "github.com/gorilla/mux"
	rovyapi "go.rovy.net/api"
)

type Server struct {
	node   rovyapi.NodeAPI
	logger *log.Logger
}

func NewServer(node rovyapi.NodeAPI, logger *log.Logger) *Server {
	s := &Server{node, logger}
	return s
}

// TODO: also serve /v0/start
func (s *Server) Serve(lis net.Listener) {
	router := mux.NewRouter()

	router.HandleFunc("/v0/info", s.serveInfo)
	router.HandleFunc("/v0/stop", s.serveStop)
	router.HandleFunc("/v0/fcnet/start", s.serveFcnetStart) // not part of THE api
	router.HandleFunc("/v0/peer/status", s.servePeerStatus)
	router.HandleFunc("/v0/peer/listen", s.servePeerListen)
	// router.HandleFunc("/v0/peer/close", s.servePeerClose)
	router.HandleFunc("/v0/peer/connect", s.servePeerConnect)
	// router.HandleFunc("/v0/peer/disconnect", s.servePeerDisconnect)

	// router.HandleFunc("/v0/discovery/status", s.serveDiscoveryStatus)
	router.HandleFunc("/v0/discovery/linklocal/start", s.serveDiscoveryLinkLocalStart)
	router.HandleFunc("/v0/discovery/linklocal/stop", s.serveDiscoveryLinkLocalStop)

	srv := &http.Server{Handler: router}
	if err := srv.Serve(lis); err != nil {
		// return err
	}
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Printf("api: request %s -> error: %s", r.RequestURI, err)
	w.WriteHeader(http.StatusInternalServerError)
}

func (s *Server) serveInfo(w http.ResponseWriter, r *http.Request) {
	ni, _ := s.node.Info()
	out, err := json.Marshal(ni)
	if err != nil {
		s.writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	out = append(out, 0x0a) // newline
	_, _ = w.Write(out)

	s.logger.Printf("api request %s -> ok", r.RequestURI)
}

func (s *Server) serveStop(w http.ResponseWriter, r *http.Request) {
	// s.node.Stop()
	w.WriteHeader(http.StatusOK)
	s.logger.Printf("api request %s -> ok", r.RequestURI)
}
