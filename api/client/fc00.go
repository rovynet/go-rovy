package rovyapic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	rovyapi "go.rovy.net/api"
)

type Fc00Client Client

// TODO: put timeouts on Post request and socket listener
func (c *Fc00Client) Start(tunfd *os.File) error {
	dir, err := os.MkdirTemp(os.TempDir(), "rovy0-*")
	if err != nil {
		return fmt.Errorf("mkdirtemp: %s", err)
	}
	defer os.RemoveAll(dir)

	sa := filepath.Join(dir, "v0-fc00-start.sock")
	go func() {
		if err := sendFD(sa, int(tunfd.Fd())); err != nil {
			c.logger.Printf("sendFD: %s", err)
		}
	}()

	params := struct{ Socket string }{sa}
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("params: %s", err)
	}
	// TODO: check for status code
	_, err = c.http.Post("http://unix/v0/fc00/start", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("http: %s", err)
	}

	return nil
}

func (c *Fc00Client) NodeAPI() rovyapi.NodeAPI {
	return (*Client)(c)
}

var _ rovyapi.Fc00API = &Fc00Client{}

func sendFD(socket string, fd int) error {
	l, err := net.Listen("unix", socket)
	if err != nil {
		return fmt.Errorf("listen: %s", err)
	}
	defer l.Close()

	conn, err := l.Accept()
	if err != nil {
		return fmt.Errorf("accept: %s", err)
	}
	defer conn.Close()

	apifd, err := conn.(*net.UnixConn).File()
	if err != nil {
		return fmt.Errorf("unixconn: %s", err)
	}

	msg := unix.UnixRights(fd) // 4 bytes per fd
	if err := unix.Sendmsg(int(apifd.Fd()), nil, msg, nil, 0); err != nil {
		return fmt.Errorf("sendmsg: %s", err)
	}

	return nil
}
