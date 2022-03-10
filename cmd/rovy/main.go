package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"

	multibase "github.com/multiformats/go-multibase"
	cli "github.com/urfave/cli/v2"
	rovy "go.rovy.net"
	rovyapic "go.rovy.net/api/client"
	rovyapis "go.rovy.net/api/server"
	rovynode "go.rovy.net/node"
)

const DefaultSocket = "/tmp/rovy.sock"

func main() {
	app := &cli.App{
		Name:    "rovy",
		Version: "0.0.0",
		Commands: []*cli.Command{{
			Name:   "run",
			Action: runCmd,
		}, {
			Name:   "info",
			Action: infoCmd,
		}},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("error: %s\n", err)
	}
}

func isatty_unix(fd *os.File) bool {
	stat, _ := fd.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return false
	} else {
		return true
	}
}

func runCmd(c *cli.Context) error {
	logger := log.New(c.App.ErrWriter, "", log.Ltime|log.Lshortfile)

	var privkey rovy.PrivateKey
	var sock string

	if isatty_unix(os.Stdin) {
		logger.Printf("starting with random ephemeral config")
		privkey2, err := rovy.GeneratePrivateKey()
		if err != nil {
			return err
		}
		privkey = privkey2
		sock = DefaultSocket
	} else {
		logger.Printf("starting with config from stdin")
		input := new(struct {
			privkey string
			sock    string
		})
		if err := json.NewDecoder(os.Stdin).Decode(input); err != nil {
			return err
		}
		_, privbytes, err := multibase.Decode(input.privkey)
		if err != nil {
			return err
		}
		privkey = rovy.NewPrivateKey(privbytes)
		sock = input.sock
	}

	node := rovynode.NewNode(privkey, logger)
	logger.Printf("we are /rovy/%s", node.PeerID())

	apilis, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	api := rovyapis.NewServer(node, logger)
	go api.Serve(apilis)
	logger.Printf("api ready on http://unix%s", sock)

	select {}

	return nil
}

func infoCmd(c *cli.Context) error {
	logger := log.New(c.App.ErrWriter, "", log.Ltime|log.Lshortfile)

	api := rovyapic.NewClient(DefaultSocket, logger)

	ni, err := api.Info()
	if err != nil {
		return err
	}

	// logger.Printf("infoCmd: %+v", ni)

	fmt.Fprintf(os.Stdout, "PeerID: %s\n", ni.PeerID)

	return nil
}
