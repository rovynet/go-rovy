package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	cli "github.com/urfave/cli/v2"

	rovy "go.rovy.net"
	rovyapic "go.rovy.net/api/client"
	rconfig "go.rovy.net/api/config"
	rnodecfg "go.rovy.net/api/config/nodecfg"
	rovyapis "go.rovy.net/api/server"
	rovynode "go.rovy.net/node"
)

const KeyfileName = "keyfile.toml"
const ConfigName = "config.toml"

var startCmd = &cli.Command{
	Name:   "start",
	Action: startCmdFunc,
	Flags: []cli.Flag{
		directoryFlag,
		socketFlag,
		&cli.StringFlag{
			Name:    "keyfile",
			Aliases: []string{"K"},
			Value:   filepath.Join(DefaultDirectory, KeyfileName),
			// example: mWNcIKVDFPG5k7bucZ5nf98aVBXKporSBfF4YnGgtBM0
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"C"},
			Value:   filepath.Join(DefaultDirectory, ConfigName),
		},
	},
}

func startCmdFunc(c *cli.Context) error {
	if c.IsSet("directory") && (c.IsSet("socket") || c.IsSet("keyfile") || c.IsSet("config")) {
		return exitErr("The --directory flag cannot be used alongside --socket, --keyfile or --config")
	}

	logger := newLogger(c)

	var socket, keyfile, config string
	var ephemeral, stdin bool

	directory, err := homedir.Expand(c.String("directory"))
	if err != nil {
		return exitErr("homedir: %s", err)
	}

	if c.IsSet("directory") {
		socket = filepath.Join(directory, SocketName)
		keyfile = filepath.Join(directory, KeyfileName)
		config = filepath.Join(directory, ConfigName)
	} else {
		socket, err = homedir.Expand(c.String("socket"))
		if err != nil {
			return exitErr("homedir: %s", err)
		}

		keyfile = c.String("keyfile")
		ephemeral = keyfile == "@"
		stdin = keyfile == "-"
		if !ephemeral && !stdin {
			keyfile, err = homedir.Expand(keyfile)
			if err != nil {
				return exitErr("homedir: %s", err)
			}
		}

		config, err = homedir.Expand(c.String("config"))
		if err != nil {
			return exitErr("homedir: %s", err)
		}
	}

	privkey, err := readPrivateKey(keyfile, os.Stdin)
	if err != nil {
		return exitErr("privkey: %s", err)
	}

	node := rovynode.NewNode(privkey, logger)
	logger.Printf("we are /rovy/%s", node.PeerID())

	if err := startAPI(node, socket); err != nil {
		return exitErr("api: %s", err)
	}
	logger.Printf("api socket ready at http:%s", socket)

	if !stdin {
		if !ephemeral {
			if err := configureNode(node, socket, config); err != nil {
				return exitErr("config: %s", err)
			}
		} else {
			if err := configureNodeDefault(node, socket); err != nil {
				return exitErr("config: %s", err)
			}
		}
	}

	if _, err := node.Start(); err != nil {
		return exitErr("node: %s", err)
	}

	select {
	// XXX shutdown needs to break this select
	}
}

func readPrivateKey(keyfile string, stdin io.Reader) (privkey rovy.PrivateKey, err error) {
	switch keyfile {
	case "@":
		privkey, err = rovy.GeneratePrivateKey()
		if err != nil {
			return privkey, fmt.Errorf("error generating private key: %s", err)
		}
		return privkey, nil
	case "-":
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return privkey, fmt.Errorf("error reading from stdin: %s", err)
		}
		privkey, err = rovy.ParsePrivateKey(line)
		if err != nil {
			return privkey, fmt.Errorf("error decoding private key: %s", err)
		}
		return privkey, nil
	default:
		kf, err := rconfig.LoadKeyfile(keyfile)
		if err != nil {
			return privkey, fmt.Errorf("keyfile: %s", err)
		}
		return kf.PrivateKey, nil
	}
}

func startAPI(node *rovynode.Node, socket string) error {
	if err := checkSocket(socket); err != nil {
		return fmt.Errorf("failed to check socket %s: %s", socket, err)
	}
	apilis, err := net.Listen("unix", socket)
	if err != nil {
		return fmt.Errorf("failed to start socket listener: %s", err)
	}
	api := rovyapis.NewServer(node, node.Log())
	go api.Serve(apilis)
	return nil
}

func configureNode(node *rovynode.Node, socket, config string) error {
	cfg, err := rconfig.LoadConfig(config)
	if err != nil {
		return fmt.Errorf("failed to load config: %s", err)
	}
	client := rovyapic.NewClient(socket, node.Log())
	nc := &rnodecfg.NodeConfig{API: client, Logger: node.Log()}
	if err = nc.ConfigureAll(cfg, node); err != nil {
		return err
	}
	return nil
}

func configureNodeDefault(node *rovynode.Node, socket string) error {
	cfg := rconfig.DefaultConfig()
	client := rovyapic.NewClient(socket, node.Log())
	nc := &rnodecfg.NodeConfig{API: client, Logger: node.Log()}
	if err := nc.ConfigureAll(cfg, node); err != nil {
		return err
	}
	return nil
}

func checkSocket(socket string) error {
	if _, err := os.Stat(socket); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	sockconn, err := net.Dial("unix", socket)
	if err != nil {
		if strings.Contains(err.Error(), "refused") {
			if err := os.Remove(socket); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	// someone else using it
	sockconn.Close()
	return fmt.Errorf("used by someone else")
}

var stopCmd = &cli.Command{
	Name:   "stop",
	Action: stopCmdFunc,
	Flags: []cli.Flag{
		directoryFlag,
		socketFlag,
	},
}

func stopCmdFunc(c *cli.Context) error {
	logger := newLogger(c)

	socket, err := getSocket(c)
	if err != nil {
		return exitErr("getsocket: %s", err)
	}

	api := rovyapic.NewClient(socket, logger)
	_, err = api.Stop()
	if err != nil {
		return exitErr("api: %s", err)
	}

	return exitErr("node operation is paused, but exiting isn't implemented yet")
}
