package rovyapic

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"

	rovyapi "go.rovy.net/api"
)

type Client struct {
	sock   string
	logger *log.Logger
	http   http.Client
}

func NewClient(sock string, logger *log.Logger) *Client {
	hc := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", sock)
			},
		},
	}
	c := &Client{sock, logger, hc}
	return c
}

func (c *Client) Info() (ni rovyapi.NodeInfo, err error) {
	res, err := c.http.Get("http://unix/v0/info")
	if err != nil {
		return ni, err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return ni, err
	}

	err = json.Unmarshal(body, &ni)
	if err != nil {
		return ni, err
	}

	return ni, err
}

func (c *Client) Stop() error {
	res, err := c.http.Get("http://unix/v0/stop")
	if err != nil {
		return err
	}
	_ = res

	return nil
}

var _ rovyapi.NodeAPI = &Client{}
