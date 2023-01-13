package main

import (
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	cli "github.com/urfave/cli/v2"

	rovy "go.rovy.net"
	rconfig "go.rovy.net/api/config"
)

var initCmd = &cli.Command{
	Name:   "init",
	Action: initCmdFunc,
	Flags: []cli.Flag{
		directoryFlag,
		&cli.StringFlag{
			Name:    "keyfile",
			Aliases: []string{"K"},
			Value:   filepath.Join(DefaultDirectory, KeyfileName),
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"C"},
			Value:   filepath.Join(DefaultDirectory, ConfigName),
		},
	},
}

func initCmdFunc(c *cli.Context) error {
	if c.IsSet("directory") && (c.IsSet("keyfile") || c.IsSet("config")) {
		return exitErr("The --directory flag cannot be used alongside --keyfile or --config")
	}

	logger := newLogger(c)

	var kfpath, cfgpath string
	directory, err := homedir.Expand(c.String("directory"))
	if err != nil {
		return exitErr("homedir: %s", err)
	}
	if c.IsSet("directory") {
		kfpath = filepath.Join(directory, KeyfileName)
		cfgpath = filepath.Join(directory, ConfigName)
	} else {
		kfpath, err = homedir.Expand(c.String("keyfile"))
		if err != nil {
			return exitErr("homedir: %s", err)
		}
		cfgpath, err = homedir.Expand(c.String("config"))
		if err != nil {
			return exitErr("homedir: %s", err)
		}
	}

	_, err = os.Stat(kfpath)
	if err == nil {
		return exitErr("keyfile already exists: %s", kfpath)
	}
	_, err = os.Stat(cfgpath)
	if err == nil {
		return exitErr("config already exists: %s", cfgpath)
	}

	cfg := rconfig.DefaultConfig()
	kf := &rconfig.Keyfile{PrivateKey: rovy.MustGeneratePrivateKey()}
	kf.PeerID = rovy.NewPeerID(kf.PrivateKey.PublicKey())
	kf.IPAddr = kf.PrivateKey.PublicKey().IPAddr()

	if err = os.MkdirAll(filepath.Dir(kfpath), 0700); err != nil {
		return exitErr("failed to create dir: %s", err)
	}
	if err := rconfig.SaveKeyfile(kfpath, kf); err != nil {
		return exitErr("failed to write keyfile: %s", err)
	}
	logger.Printf("Wrote %s", kfpath)

	if err = os.MkdirAll(filepath.Dir(cfgpath), 0700); err != nil {
		return exitErr("failed to create dir: %s", err)
	}
	if err = rconfig.SaveConfig(cfgpath, cfg); err != nil {
		return exitErr("failed to write config: %s", err)
	}
	logger.Printf("Wrote %s", cfgpath)

	return nil
}
