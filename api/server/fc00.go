package rovyapis

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"golang.org/x/sys/unix"

	rovy "go.rovy.net"
	rovyfc00 "go.rovy.net/fc00"
	rovynode "go.rovy.net/node"
)

func (s *Server) serveFc00Start(w http.ResponseWriter, r *http.Request) {
	params := struct{ Socket string }{}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		s.writeError(w, r, fmt.Errorf("params: %s", err))
		return
	}

	conn, err := net.Dial("unix", params.Socket)
	if err != nil {
		s.writeError(w, r, fmt.Errorf("dial: %s", err))
		return
	}

	apifd, err := conn.(*net.UnixConn).File()
	if err != nil {
		s.writeError(w, r, fmt.Errorf("apifd: %s", err))
		return
	}

	var fds []int

	oob := make([]byte, unix.CmsgSpace(1*4))
	_, oobn, _, _, err := unix.Recvmsg(int(apifd.Fd()), nil, oob, 0)
	if err != nil {
		s.writeError(w, r, fmt.Errorf("recvmsg: %s", err))
		return
	}

	msgs, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		s.writeError(w, r, fmt.Errorf("ParseSocketControlMessage: %s", err))
		return
	}
	if len(msgs) != 1 {
		s.writeError(w, r, fmt.Errorf("recvmsg got more than one UnixRights message"))
		return
	}
	fds, err = unix.ParseUnixRights(&msgs[0])
	if err != nil {
		s.writeError(w, r, fmt.Errorf("ParseUnixRights: %s", err))
		return
	}
	if len(fds) != 1 {
		s.writeError(w, r, fmt.Errorf("recvmsg got more than one file descriptor"))
		return
	}

	// TODO: check if the device has correct address and mtu
	tunif, err := rovyfc00.FileTUN(fds[0])
	if err != nil {
		s.writeError(w, r, fmt.Errorf("tun: %s", err))
		return
	}

	node := s.node.(*rovynode.Node)

	fc00 := rovyfc00.NewFc00(node, tunif, node.Routing())
	if err := fc00.Start(rovy.UpperMTU); err != nil {
		s.writeError(w, r, fmt.Errorf("start: %s", err))
		return
	}

	w.WriteHeader(http.StatusOK)
	s.logger.Printf("api request %s -> ok", r.RequestURI)
}
