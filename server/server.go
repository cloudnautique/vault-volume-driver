package server

import (
	"net/http"

	"github.com/Sirupsen/logrus"
)

var listenAddress = ":8080"
var vaultClient *VaultClient

func startServer() error {
	var err error

	vaultClient, err = NewVaultClient("http://127.0.0.1:8200", "password", "demo")
	if err != nil {
		return err
	}

	router := NewRouter()
	logrus.Infof("Starting server on: %s", listenAddress)
	return http.ListenAndServe(listenAddress, router)
}
