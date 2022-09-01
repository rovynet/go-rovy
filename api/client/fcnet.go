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

type FcnetClient Client

// TODO: put timeouts on Post request and socket listener
func (c *FcnetClient) Start(tunfd *os.File) error {
	dir, err := os.MkdirTemp(os.TempDir(), "rovy0-*")
	if err != nil {
		return fmt.Errorf("mkdirtemp: %s", err)
	}
	defer os.RemoveAll(dir)

	sa := filepath.Join(dir, "v0-fcnet-start.sock")
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
	_, err = c.http.Post("http://unix/v0/fcnet/start", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("http: %s", err)
	}

	return nil
}

func (c *FcnetClient) NodeAPI() rovyapi.NodeAPI {
	return (*Client)(c)
}

var _ rovyapi.FcnetAPI = &FcnetClient{}

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

	msg := []byte{0x42} // out-of-band data must be sent with some actual data...
	oob := unix.UnixRights(fd)
	if err := unix.Sendmsg(int(apifd.Fd()), msg, oob, nil, 0); err != nil {
		return fmt.Errorf("sendmsg: %s", err)
	}

	return nil
}
