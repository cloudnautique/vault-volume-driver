package server

import (
	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

func Command() cli.Command {
	return cli.Command{
		Name:   "server",
		Usage:  "Provides endpoint for volume driver to request tokens for Vault",
		Action: StartServer,
	}
}

func StartServer(c *cli.Context) error {
	err := startServer()
	if err != nil {
		logrus.Error(err)
	}
	return err
}
