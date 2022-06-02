package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	multibase "github.com/multiformats/go-multibase"
	cli "github.com/urfave/cli/v2"

	rovy "go.rovy.net"
	rovyapic "go.rovy.net/api/client"
	rovyapis "go.rovy.net/api/server"
	rovyfc00 "go.rovy.net/fc00"
	rovynode "go.rovy.net/node"
)

const KeyfileName = "private.key"
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

	privkey, err := readPrivateKey(keyfile, os.Stdin, logger)
	if err != nil {
		return exitErr("privkey: %s", err)
	}

	node := rovynode.NewNode(privkey, logger)
	logger.Printf("we are /rovy/%s", node.PeerID())

	if err := checkSocket(socket); err != nil {
		return exitErr("failed to check socket %s: %s", socket, err)
	}

	apilis, err := net.Listen("unix", socket)
	if err != nil {
		return exitErr("failed to start socket listener: %s", err)
	}
	api := rovyapis.NewServer(node, logger)
	go api.Serve(apilis)
	logger.Printf("api socket ready at http:%s", socket)

	// if ephemeral || stdin {
	// 	logger.Printf("ignoring configuration")
	// } else {
	// 	logger.Printf("TODO: reading configuration file is not implemented yet")
	// }
	_ = config

	// if !ephemeral && !stdin {
	if !stdin {
		if err = configureFc00(socket, node, logger); err != nil {
			return exitErr("failed to configure fc00: %s", err)
		}
	}

	select {
	// XXX shutdown needs to break this select
	}

	return nil
}

func readPrivateKey(keyfile string, stdin io.Reader, logger *log.Logger) (privkey rovy.PrivateKey, err error) {
	switch keyfile {
	case "@":
		logger.Printf("starting with ephemeral private key and no configuration")
		privkey, err = rovy.GeneratePrivateKey()
		if err != nil {
			return privkey, fmt.Errorf("error generating private key: %s", err)
		}
	case "-":
		logger.Printf("starting with private key from stdin and no configuration")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return privkey, fmt.Errorf("error reading from stdin: %s", err)
		}
		_, b, err := multibase.Decode(line)
		if err != nil {
			return privkey, fmt.Errorf("error decoding multibase private key: %s", err)
		}
		privkey = rovy.NewPrivateKey(b)
	default:
		return privkey, fmt.Errorf("TODO: reading from keyfile not implemented yet, use `-K -` or `-K @`")
	}

	// Just for printf debugging:
	// privstr, err := multibase.Encode(multibase.Base64, privkey.BytesSlice())
	// if err != nil {
	// 	return privkey, fmt.Errorf("error encoding multibase private key: %s", err)
	// }
	// logger.Printf("key: %s", privstr)

	return privkey, nil
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

func configureFc00(socket string, node *rovynode.Node, logger *log.Logger) error {
	ip6a := node.PeerID().PublicKey().Addr()
	tunif, err := rovyfc00.NetworkManagerTun(rovyfc00.TunIfname, ip6a, rovy.UpperMTU, logger)
	if err != nil {
		return fmt.Errorf("networkmanager: %s", err)
	}

	tunfd := tunif.File()

	api := rovyapic.NewClient(socket, logger)
	err = (*rovyapic.Fc00Client)(api).Start(tunfd)
	if err != nil {
		return fmt.Errorf("api: %s", err)
	}

	logger.Printf("started fc00 endpoint %s using NetworkManager", ip6a)

	return nil
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
	err = api.Stop()
	if err != nil {
		return exitErr("api: %s", err)
	}

	return exitErr("TODO: shutdown is not yet implemented (and neither are api error responses)")
}
