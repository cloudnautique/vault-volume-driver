package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher/api"
)

func CreateTokenRequest(rw http.ResponseWriter, req *http.Request) (int, error) {
	apiContext := api.GetApiContext(req)

	message := &VaultTokenInput{}

	jsonDecoder := json.NewDecoder(req.Body)

	err := jsonDecoder.Decode(message)
	if err != nil {
		logrus.Debugf("Error: %s", err.Error())
		return http.StatusBadRequest, err
	}

	resp, err := vaultClient.NewWrappedVaultToken(policiesList(message.Policies))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	logrus.Infof("Response: %#v", resp)
	apiContext.Write(NewVaultTokenResponse(resp))

	return http.StatusOK, nil
}

func policiesList(policies string) []string {
	return strings.Split(policies, ",")
}
