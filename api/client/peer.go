package rovyapic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	rovy "go.rovy.net"
	rovyapi "go.rovy.net/api"
)

type PeerClient Client

func (c *PeerClient) Status() rovyapi.PeerStatus {
	return rovyapi.PeerStatus{}
}

func (c *PeerClient) Enable(ma rovy.Multiaddr) (pd rovyapi.PeerDialer, err error) {
	params := struct{ Protocol rovy.Multiaddr }{ma}
	reqbody, err := json.Marshal(&params)
	if err != nil {
		return pd, err
	}
	res, err := c.http.Post("http://unix/v0/peer/enable", "application/json", bytes.NewReader(reqbody))
	if err != nil {
		return pd, err
	}
	if res.StatusCode != http.StatusOK {
		return pd, fmt.Errorf("http: %s", res.Status)
	}

	if err := json.NewDecoder(res.Body).Decode(&pd); err != nil {
		return pd, err
	}
	return pd, err
}

func (c *PeerClient) Listen(ma rovy.Multiaddr) (pl rovyapi.PeerListener, err error) {
	params := struct{ Addr rovy.Multiaddr }{ma}
	reqbody, err := json.Marshal(&params)
	if err != nil {
		return pl, err
	}

	res, err := c.http.Post("http://unix/v0/peer/listen", "application/json", bytes.NewReader(reqbody))
	if err != nil {
		return pl, err
	}
	if res.StatusCode != http.StatusOK {
		return pl, fmt.Errorf("http: %s", res.Status)
	}

	if err := json.NewDecoder(res.Body).Decode(&pl); err != nil {
		return pl, err
	}
	return pl, err
}

func (c *PeerClient) Connect(ma rovy.Multiaddr) (rovyapi.PeerInfo, error) {
	return rovyapi.PeerInfo{}, nil
}

func (c *PeerClient) NodeAPI() rovyapi.NodeAPI {
	return (*Client)(c)
}

var _ rovyapi.PeerAPI = &PeerClient{}
