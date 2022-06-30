package rovyapis

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"golang.org/x/sys/unix"

	rovy "go.rovy.net"
	fcnet "go.rovy.net/fcnet"
	rovynode "go.rovy.net/node"
)

func (s *Server) serveFcnetStart(w http.ResponseWriter, r *http.Request) {
	params := struct{ Socket string }{}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		s.writeError(w, r, fmt.Errorf("params: %s", err))
		return
	}

	fd, err := receiveFD(params.Socket)
	if err != nil {
		s.writeError(w, r, err)
		return
	}

	// TODO: check if the device has correct address and mtu
	tunif, err := fcnet.FileTUN(fd)
	if err != nil {
		s.writeError(w, r, fmt.Errorf("tun: %s", err))
		return
	}

	node := s.node.(*rovynode.Node)

	fc := fcnet.NewFcnet(node, tunif)
	if err := fc.Start(rovy.UpperMTU); err != nil {
		s.writeError(w, r, fmt.Errorf("start: %s", err))
		return
	}

	w.WriteHeader(http.StatusOK)
	s.logger.Printf("api request %s -> ok", r.RequestURI)
}

func receiveFD(socket string) (int, error) {
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return 0, fmt.Errorf("dial: %s", err)
	}

	apifd, err := conn.(*net.UnixConn).File()
	if err != nil {
		return 0, fmt.Errorf("apifd: %s", err)
	}

	oob := make([]byte, unix.CmsgSpace(1*4))
	_, oobn, _, _, err := unix.Recvmsg(int(apifd.Fd()), nil, oob, 0)
	if err != nil {
		return 0, fmt.Errorf("recvmsg: %s", err)
	}

	msgs, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return 0, fmt.Errorf("ParseSocketControlMessage: %s", err)
	}
	if len(msgs) != 1 {
		return 0, fmt.Errorf("recvmsg got more than one UnixRights message")
	}
	fds, err := unix.ParseUnixRights(&msgs[0])
	if err != nil {
		return 0, fmt.Errorf("ParseUnixRights: %s", err)
	}
	if len(fds) != 1 {
		return 0, fmt.Errorf("recvmsg got more than one file descriptor")
	}

	return fds[0], nil
}
