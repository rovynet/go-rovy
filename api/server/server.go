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

	srv := &http.Server{Handler: router}
	srv.Serve(lis)
}

func (s *Server) serveInfo(w http.ResponseWriter, r *http.Request) {
	ni, _ := s.node.Info()
	out, err := json.Marshal(ni)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.logger.Printf("api: request %s -> error: %s", r.URL.Path, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	out = append(out, 0x0a) // newline
	_, _ = w.Write(out)

	s.logger.Printf("api request %s -> ok", r.RequestURI)
}
