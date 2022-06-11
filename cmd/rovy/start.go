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
	cli "github.com/urfave/cli/v2"

	rovy "go.rovy.net"
	rovyapic "go.rovy.net/api/client"
	rovyapis "go.rovy.net/api/server"
	rovycfg "go.rovy.net/cmd/rovy/config"
	rovyfc00 "go.rovy.net/fc00"
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

	if !stdin {
		var cfg *rovycfg.Config
		if !ephemeral {
			cfg, err = rovycfg.LoadConfig(config)
			if err != nil {
				return exitErr("failed to load config: %s", err)
			}
		} else {
			cfg = rovycfg.DefaultConfig()
		}

		if err = configurePeering(rovyapic.NewClient(socket, logger), cfg, node); err != nil {
			return exitErr("failed to configure peering: %s", err)
		}

		if err = configureFc00(rovyapic.NewClient(socket, logger), cfg, node); err != nil {
			return exitErr("failed to configure fc00: %s", err)
		}
	}

	select {
	// XXX shutdown needs to break this select
	}
}

func readPrivateKey(keyfile string, stdin io.Reader, logger *log.Logger) (privkey rovy.PrivateKey, err error) {
	switch keyfile {
	case "@":
		privkey, err = rovy.GeneratePrivateKey()
		if err != nil {
			return privkey, fmt.Errorf("error generating private key: %s", err)
		}
	case "-":
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return privkey, fmt.Errorf("error reading from stdin: %s", err)
		}
		privkey, err = rovy.ParsePrivateKey(line)
		if err != nil {
			return privkey, fmt.Errorf("error decoding private key: %s", err)
		}
	default:
		kf, err := rovycfg.LoadKeyfile(keyfile)
		if err != nil {
			return privkey, fmt.Errorf("keyfile: %s", err)
		}
		return kf.PrivateKey, nil
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

// TODO: do the actual configuration using api/client module
func configurePeering(api *rovyapic.Client, cfg *rovycfg.Config, node *rovynode.Node) error {
	for _, protos := range cfg.Peer.Dialers {
		_, err := api.Peer().Enable(protos)
		if err != nil {
			return err
		}
	}
	for _, addr := range cfg.Peer.Listeners {
		_, err := api.Peer().Listen(addr)
		if err != nil {
			return err
		}
	}
	return nil
}

// TODO: make use of actual config
func configureFc00(api *rovyapic.Client, cfg *rovycfg.Config, node *rovynode.Node) error {
	if !cfg.Fc00.Enabled {
		return nil
	}

	tunif, err := rovyfc00.NetworkManagerTun(cfg.Fc00.Ifname, node.IPAddr(), rovy.UpperMTU, node.Log())
	if err != nil {
		return fmt.Errorf("networkmanager: %s", err)
	}

	tunfd := tunif.File()

	err = api.Fc00().Start(tunfd)
	if err != nil {
		return fmt.Errorf("api: %s", err)
	}

	node.Log().Printf("started fc00 endpoint %s using NetworkManager", node.IPAddr())

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
