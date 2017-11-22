package server

import (
	"net/http"

	"fmt"
	"github.com/Sirupsen/logrus"
)

var listenAddress = ":8080"
var vaultClient *VaultClient

// Config contains config info for server setup.
type Config struct {
	VaultURL   string
	VaultRole  string
	VaultToken string
}

type ConfigError struct {
	errorField string
}

func (c ConfigError) Error() string {
	return fmt.Sprintf("Invalid Config: %s is not set correctly.", c.errorField)
}

func startServer(config *Config) error {
	var err error

	if err = config.ValidateConfig(); err != nil {
		return err
	}

	vaultClient, err = NewVaultClient(config.VaultURL, config.VaultToken, config.VaultRole)
	if err != nil {
		return err
	}

	router := NewRouter()
	logrus.Infof("Starting server on: %s", listenAddress)
	return http.ListenAndServe(listenAddress, router)
}

func (c *Config) ValidateConfig() error {
	if c.VaultRole == "" {
		return ConfigError{errorField: "VaultRole"}
	}

	if c.VaultToken == "" {
		return ConfigError{errorField: "VaultToken"}
	}

	if c.VaultURL == "" {
		return ConfigError{errorField: "VaultURL"}
	}

	return nil
}
