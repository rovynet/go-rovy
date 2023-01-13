package rovyapic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	rovyapi "go.rovy.net/api"
)

type DiscoveryClient Client

func (c *DiscoveryClient) Status() (rovyapi.DiscoveryStatus, error) {
	return rovyapi.DiscoveryStatus{}, nil
}

func (c *DiscoveryClient) StartLinkLocal(opts rovyapi.DiscoveryLinkLocal) error {
	params := struct{ Interval string }{opts.Interval.String()}
	reqbody, err := json.Marshal(&params)
	if err != nil {
		return err
	}

	res, err := c.http.Post("http://unix/v0/discovery/linklocal/start", "application/json", bytes.NewReader(reqbody))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("http: %s", res.Status)
	}

	return nil
}

func (c *DiscoveryClient) StopLinkLocal() error {
	res, err := c.http.Post("http://unix/v0/discovery/linklocal/stop", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("http: %s", res.Status)
	}

	return nil
}
