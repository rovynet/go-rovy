package main

import (
	"fmt"
	"os"

	cli "github.com/urfave/cli/v2"
	rovy "go.rovy.net"
	rovyapic "go.rovy.net/api/client"
)

var peerCmd = &cli.Command{
	Name: "peer",
	Subcommands: []*cli.Command{
		{
			Name:   "status",
			Action: peerStatusCmdFunc,
		},
		{
			Name:   "listen",
			Action: peerListenCmdFunc,
		},
		{
			Name:   "connect",
			Action: peerConnectCmdFunc,
		},
		{
			Name:   "policy",
			Action: peerPolicyCmdFunc,
		},
	},
}

func peerStatusCmdFunc(c *cli.Context) error {
	logger := newLogger(c)
	socket, err := getSocket(c)
	if err != nil {
		return exitErr("getsocket: %s", err)
	}

	api := rovyapic.NewClient(socket, logger)
	status := api.Peer().Status()

	fmt.Fprintf(os.Stdout, "Status: %#v\n", status)

	return nil
}

func peerListenCmdFunc(c *cli.Context) error {
	logger := newLogger(c)
	socket, err := getSocket(c)
	if err != nil {
		return exitErr("getsocket: %s", err)
	}

	if c.NArg() == 0 {
		return exitErr("expecting multiaddr argument")
	}
	for i := 0; i < c.NArg(); i++ {
		maddr, err := rovy.ParseMultiaddr(c.Args().Get(i))
		if err != nil {
			return exitErr("multiaddr: %s", err)
		}

		api := rovyapic.NewClient(socket, logger)
		pl, err := api.Peer().Listen(maddr)

		fmt.Fprintf(os.Stdout, "Listener: %#v\n", pl)
	}

	return nil
}

func peerConnectCmdFunc(c *cli.Context) error {
	return exitErr("TODO: connect is not yet implemented")
}

func peerPolicyCmdFunc(c *cli.Context) error {
	return exitErr("TODO: policy is not yet implemented")
}
