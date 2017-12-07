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

	logrus.Debugf("Response: %#v", resp)
	apiContext.Write(NewVaultTokenResponse(resp))

	return http.StatusOK, nil
}

func RevokeTokenRequest(rw http.ResponseWriter, req *http.Request) (int, error) {
	vte, err := newVerifiedRevokeTokenRequest(req)
	if err != nil {
		return http.StatusBadRequest, err
	}

	err = vaultClient.RevokeToken(vte.Accessor)
	if err != nil {
		logrus.Errorf("failed to revoke token: %s got: %s\n", vte.Accessor, err)
		return http.StatusBadRequest, nil
	}

	logrus.Debugf("Revoked token: %s", vte.Accessor)

	return http.StatusAccepted, nil
}

func policiesList(policies string) []string {
	return strings.Split(policies, ",")
}

func newVerifiedVaultTokenInput(req *http.Request) (*VaultTokenInput, error) {
	msg := &VaultTokenInput{}

	jsonDecoder := json.NewDecoder(req.Body)

	err := jsonDecoder.Decode(msg)
	if err != nil {
		logrus.Debugf("Error: %s", err.Error())
		return msg, err
	}

	verified, err := verifySignature(msg.HostUUID, req.Header.Get(SignatureHeaderString), msg)
	if err != nil {
		return msg, err
	}

	if verified {
		logrus.Debugf("VERIFIED: %s", verified)
		return msg, nil
	}

	return msg, fmt.Errorf("signatures did not match")
}

func newVerifiedRevokeTokenRequest(req *http.Request) (*VaultTokenExpireInput, error) {
	msg := &VaultTokenExpireInput{}

	jsonDecoder := json.NewDecoder(req.Body)

	err := jsonDecoder.Decode(msg)
	if err != nil {
		return msg, err
	}

	verified, err := verifySignature(msg.HostUUID, req.Header.Get(SignatureHeaderString), msg)
	if err != nil {
		return msg, err
	}

	if verified {
		logrus.Debugf("VERIFIED: %s", verified)
		return msg, nil
	}

	return msg, fmt.Errorf("signatures did not match")
}

func verifySignature(hostUUID, reqSignature string, msg signature.Message) (bool, error) {
	verified := false

	sigBytes, err := base64.StdEncoding.DecodeString(reqSignature)
	if err != nil {
		return verified, err
	}

	key, err := rancher.GetRancherHostPublicKey(rancherClient, hostUUID)
	if err != nil {
		return verified, err
	}

	pubKey, err := signature.LoadRSAPublicKey(key)
	if err != nil {
		return verified, err
	}

	return signature.Verify(sigBytes, msg, pubKey)
}
