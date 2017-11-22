package server

import (
	"github.com/urfave/cli"
)

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
		},
	}
}

func StartServer(c *cli.Context) error {
	var err error
	token := c.String("vault-token")

	if c.String("vault-token-file") != "" {
		loadVaultTokenFromFile(c.String("vault-token-file"))
	}

	config := &Config{
		VaultURL:   c.String("vault-url"),
		VaultRole:  c.String("vault-role"),
		VaultToken: token,
	}

	if err = config.ValidateConfig(); err == nil {
		return startServer(config)
	}

	return err
}

func loadVaultTokenFromFile(filePath string) (string, error) {
	return "", nil
}
