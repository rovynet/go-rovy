package main

import (
	cli "github.com/urfave/cli/v2"
)

var initCmd = &cli.Command{
	Name:   "init",
	Action: initCmdFunc,
}

func initCmdFunc(c *cli.Context) error {
	return exitErr("TODO: init is not yet implemented")

	// mkdir ~/.rovy
	// write private.key
	// write config.toml
}
