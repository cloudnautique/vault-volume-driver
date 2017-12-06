package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"encoding/base64"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/vault-volume-driver/rancher"
	"github.com/rancher/vault-volume-driver/signature"
)

const (
	SignatureHeaderString = "X-Vault-Driver-Signature"
)

func CreateTokenRequest(rw http.ResponseWriter, req *http.Request) (int, error) {
	apiContext := api.GetApiContext(req)

	vti, err := newVerifiedVaultTokenInput(req)
	if err != nil {
		return http.StatusBadRequest, err
	}

	resp, err := vaultClient.NewWrappedVaultToken(policiesList(vti.Policies))
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

func newVerifiedVaultTokenInput(req *http.Request) (*VaultTokenInput, error) {
	msg := &VaultTokenInput{}

	sigBytes, err := base64.StdEncoding.DecodeString(req.Header.Get(SignatureHeaderString))
	if err != nil {
		return msg, err
	}

	jsonDecoder := json.NewDecoder(req.Body)

	err = jsonDecoder.Decode(msg)
	if err != nil {
		logrus.Debugf("Error: %s", err.Error())
		return msg, err
	}

	if msg.HostUUID == "" {
		return msg, fmt.Errorf("no hostUUID sent")
	}

	key, err := rancher.GetRancherHostPublicKey(rancherClient, msg.HostUUID)
	if err != nil {
		return msg, err
	}

	pubKey, err := signature.LoadRSAPublicKey(key)
	if err != nil {
		return msg, err
	}

	verified, err := signature.Verify(sigBytes, msg, pubKey)
	if err != nil {
		return msg, err
	}

	if verified {
		logrus.Debugf("VERIFIED: %s", verified)
		return msg, nil
	}

	return msg, fmt.Errorf("signatures did not match")
}
