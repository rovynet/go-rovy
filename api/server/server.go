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

func (s *Server) Serve(lis net.Listener) {
	router := mux.NewRouter()

	router.HandleFunc("/v0/info", s.serveInfo)
	router.HandleFunc("/v0/stop", s.serveStop)
	router.HandleFunc("/v0/fc00/start", s.serveFc00Start) // not part of THE api

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
