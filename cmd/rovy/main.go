package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	cli "github.com/urfave/cli/v2"

	rovyapic "go.rovy.net/api/client"
)

const DefaultDirectory = "~/.rovy"
const SocketName = "api.sock"

var app = &cli.App{
	Name:    "rovy",
	Version: "0.0.0",
	Commands: []*cli.Command{
		initCmd,
		startCmd,
		infoCmd,
		stopCmd,
		peerCmd,
	},
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Printf("error: %s\n", err)
	}
}

var directoryFlag = &cli.StringFlag{
	Name:    "directory",
	Aliases: []string{"D"},
	Value:   DefaultDirectory,
}

var socketFlag = &cli.StringFlag{
	Name:    "socket",
	Aliases: []string{"S"},
	Value:   filepath.Join(DefaultDirectory, SocketName),
}

func newLogger(c *cli.Context) *log.Logger {
	return log.New(c.App.ErrWriter, "", log.Ltime|log.Lshortfile)
}

func exitErr(format string, a ...any) cli.ExitCoder {
	return cli.Exit(fmt.Sprintf(format, a...), 1)
}

var infoCmd = &cli.Command{
	Name:   "info",
	Action: infoCmdFunc,
	Flags: []cli.Flag{
		directoryFlag,
		socketFlag,
	},
}

func infoCmdFunc(c *cli.Context) error {
	logger := newLogger(c)

	socket, err := getSocket(c)
	if err != nil {
		return exitErr("getsocket: %s", err)
	}

	api := rovyapic.NewClient(socket, logger)
	ni, err := api.Info()
	if err != nil {
		return exitErr("api: %s", err)
	}

	// TODO: prettyprint if isatty, json otherwise
	fmt.Fprintf(os.Stdout, "PeerID: %s\n", ni.PeerID)

	return nil
}

func getSocket(c *cli.Context) (string, error) {
	var socket string
	directory, err := homedir.Expand(c.String("directory"))
	if err != nil {
		return socket, err
	}
	if c.IsSet("directory") {
		socket = filepath.Join(directory, SocketName)
	} else {
		socket = c.String("socket")
		socket, err = homedir.Expand(socket)
		if err != nil {
			return socket, err
		}
	}
	return socket, nil
}

// func isatty_unix(fd *os.File) bool {
// 	stat, _ := fd.Stat()
// 	if (stat.Mode() & os.ModeCharDevice) == 0 {
// 		return false
// 	} else {
// 		return true
// 	}
// }
