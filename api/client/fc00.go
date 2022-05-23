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

// TODO: put a timeout on Post request, then close the socket listener
func (c *Fc00Client) Start(tunfd *os.File) error {
	dir, err := os.MkdirTemp(os.TempDir(), "rovy0-*")
	if err != nil {
		return fmt.Errorf("mkdirtemp: %s", err)
	}
	sa := filepath.Join(dir, "v0-fc00-start.sock")
	defer os.RemoveAll(dir)

	l, err := net.Listen("unix", sa)
	if err != nil {
		return fmt.Errorf("listen: %s", err)
	}

	go func() {
		defer l.Close()
		conn, err := l.Accept()
		if err != nil {
			c.logger.Printf("accept: %s", err)
			return
		}
		defer conn.Close()

		apifd, err := conn.(*net.UnixConn).File()
		if err != nil {
			c.logger.Printf("unixconn: %s", err)
			return
		}

		msg := unix.UnixRights(int(tunfd.Fd())) // 4 bytes per fd
		if err := unix.Sendmsg(int(apifd.Fd()), nil, msg, nil, 0); err != nil {
			c.logger.Printf("sendmsg: %s", err)
			return
		}
	}()

	params := struct{ Socket string }{sa}
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("params: %s", err)
	}
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
