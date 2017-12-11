package server

import (
	"io/ioutil"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

// Command implements the server CLI options.
func Command() cli.Command {
	return cli.Command{
		Name:   "server",
		Usage:  "Provides endpoint for volume driver to request tokens for Vault",
		Action: StartServer,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "vault-url",
				Usage:  "provide http://vaulturl:port",
				EnvVar: "VAULT_ADDR",
			},
			cli.StringFlag{
				Name:   "vault-token",
				Usage:  "Vault issuing token",
				EnvVar: "VAULT_TOKEN",
			},
			cli.StringFlag{
				Name:   "vault-role",
				Usage:  "Vault issuing token role",
				EnvVar: "VAULT_ROLE",
			},
			cli.StringFlag{
				Name:  "vault-token-file",
				Usage: "file containing issuing token, takes presidence over VAULT_ADDR",
			},
			cli.StringFlag{
				Name:   "rancher-url",
				Usage:  "Rancher server url (scoped to env)",
				EnvVar: "CATTLE_URL",
			},
			cli.StringFlag{
				Name:   "rancher-access-key",
				Usage:  "Rancher access key (scoped to env)",
				EnvVar: "CATTLE_ACCESS_KEY",
			},
			cli.StringFlag{
				Name:   "rancher-secret-key",
				Usage:  "Rancher secret key",
				EnvVar: "CATTLE_SECRET_KEY",
			},
		},
	}
}

// StartServer takes the CLI options and starts a server based on the configuration.
func StartServer(c *cli.Context) error {
	var err error
	token := c.String("vault-token")

	if c.String("vault-token-file") != "" {
		logrus.Debugf("loading tokenfile: %s", c.String("vault-token-file"))
		token, err = loadVaultTokenFromFile(c.String("vault-token-file"))
		if err != nil {
			return err
		}
	}

	config := &Config{
		VaultURL:      c.String("vault-url"),
		VaultRole:     c.String("vault-role"),
		VaultToken:    token,
		RancherURL:    c.String("rancher-url"),
		RancherAccess: c.String("rancher-access-key"),
		RancherSecret: c.String("rancher-secret-key"),
	}

	if err = config.ValidateConfig(); err == nil {
		logrus.Debug("required config params sent")
		return startServer(config)
	}

	logrus.Errorf("failed to start server, bailing: %s", err)
	return err
}

func loadVaultTokenFromFile(filePath string) (string, error) {
	contentBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(contentBytes), "\n"), nil
}
