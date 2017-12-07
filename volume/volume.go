package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/moby/moby/pkg/mount"
	"github.com/rancher/go-rancher-metadata/metadata"
	"github.com/rancher/vault-volume-driver/server"
	"github.com/rancher/vault-volume-driver/signature"
)

const (
	volRoot        = "/var/lib/rancher/volumes/vault-volume-driver"
	metadataURL    = "http://169.254.169.250/2016-07-29"
	privateKeyFile = "/var/lib/rancher/etc/ssl/host.key"
)

var (
	vaultTokenServerURL = setVaultTokenServerURL()
)

type FlexVol struct{}

func setVaultTokenServerURL() string {
	envString := os.Getenv("VAULT_TOKEN_SERVER_URL")
	if envString == "" {
		envString = "http://vault-token-server:8080/v1-vault-driver/tokens"
	}

	return envString
}

func (v *FlexVol) Init() error {
	return nil
}

func (v *FlexVol) Create(options map[string]interface{}) (map[string]interface{}, error) {
	logrus.Infof("%#v", options)
	resp := options

	if name, ok := options["name"].(string); ok {
		volPath := path.Join(volRoot, name)
		logrus.Infof("volPath: %s", volPath)

		err := createTmpfs(volPath, resp)
		if err != nil {
			return resp, err
		}

		resp["device"] = newDeviceString(volPath)
	}

	return resp, nil
}

func (v *FlexVol) Delete(options map[string]interface{}) error {
	logrus.Infof("%#v", options)
	if device, ok := options["device"].(string); ok {
		return v.Detach(device)
	}
	return nil
}

func (v *FlexVol) Attach(options map[string]interface{}) (string, error) {
	dev, ok := options["device"].(string)
	if !ok {
		return dev, fmt.Errorf("could not find device key in options")
	}

	devValues, err := getDevValues(dev)
	if err != nil {
		return dev, err
	}

	host, err := getHostMetadata()
	if err != nil {
		return dev, err
	}

	policies, ok := options["policies"].(string)
	if !ok {
		return dev, fmt.Errorf("no policies were passed in driver opts, can not create token")
	}

	req := &server.VaultTokenInput{
		Policies:  policies,
		HostUUID:  host.UUID,
		TimeStamp: time.Now().UTC().String(),
	}

	token, err := makeTokenRequest(req)
	if err != nil {
		return dev, err
	}

	err = writeToken(token.Token, options)
	if err != nil {
		return dev, err
	}

	devValues.Set("accessor", token.Accessor)

	return devValues.Encode(), nil
}

func (v *FlexVol) Detach(device string) error {
	logrus.Infof("Device: %s", device)

	values, err := getDevValues(device)
	if err != nil {
		return err
	}

	if err := mount.Unmount(values.Get("device")); err != nil {
		return err
	}

	err = makeTokenRevokeRequest(values.Get("accessor"))
	if err != nil {
		return err
	}

	return os.RemoveAll(values.Get("device") + "/")
}

func (v *FlexVol) Mount(dir string, device string, params map[string]interface{}) error {
	logrus.Infof("Device: %s, dir: %s, params: %#v", device, dir, params)
	values, err := getDevValues(device)
	if err != nil {
		return err
	}
	return mount.Mount(values.Get("device"), dir, "none", "bind,rw")
}

func (v *FlexVol) Unmount(dir string) error {
	logrus.Infof("Dir: %s", dir)
	if err := mount.Unmount(dir); err != nil {
		return err
	}
	return os.RemoveAll(dir + "/")
}

func createTmpfs(dir string, options map[string]interface{}) error {
	mounted, err := mount.Mounted(dir)
	if mounted || err != nil {
		return err
	}
	mode := int(0755)
	mountOpts := "size=10m"

	if uMode, ok := options["mode"].(int); ok {
		mode = int(uMode)
	}

	if mOpts, ok := options["mountOpts"].(string); ok {
		mountOpts = mOpts
	}

	if err := os.MkdirAll(dir, os.FileMode(mode)); err != nil {
		return err
	}

	return mount.Mount("tmpfs", dir, "tmpfs", mountOpts)
}

func makeTokenRevokeRequest(accessor string) error {
	client := &http.Client{}
	vtei := &server.VaultTokenExpireInput{
		Accessor:  accessor,
		TimeStamp: time.Now().UTC().String(),
	}

	host, err := getHostMetadata()
	if err != nil {
		return err
	}

	vtei.HostUUID = host.UUID

	signature, err := getSignature(vtei)
	if err != nil {
		return err
	}

	vteiJson, err := json.Marshal(vtei)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", vaultTokenServerURL, bytes.NewBuffer(vteiJson))
	req.Header.Set(server.SignatureHeaderString, signature)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	// StatusBadRequest is returned if the vault client is already expired.
	if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusBadRequest {
		return nil
	}

	return fmt.Errorf("request status was not accepted")
}

func makeTokenRequest(tokenBody *server.VaultTokenInput) (*server.VaultIntermediateTokenResponse, error) {
	tokenResp := &server.VaultIntermediateTokenResponse{}
	client := &http.Client{}

	signature, err := getSignature(tokenBody)
	if err != nil {
		return tokenResp, err
	}

	tokenJson, err := json.Marshal(tokenBody)
	if err != nil {
		return tokenResp, err
	}

	req, err := http.NewRequest("POST", vaultTokenServerURL, bytes.NewBuffer(tokenJson))
	req.Header.Set(server.SignatureHeaderString, signature)
	if err != nil {
		return tokenResp, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return tokenResp, err
	}

	if resp.StatusCode != http.StatusOK {
		return tokenResp, fmt.Errorf("received status code %d", resp.StatusCode)
	}

	jsonDecoder := json.NewDecoder(resp.Body)
	err = jsonDecoder.Decode(tokenResp)

	return tokenResp, err
}

func getSignature(tokenBody signature.Message) (string, error) {
	content, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		return "", err
	}

	key, err := signature.LoadPrivateKeyFromString(string(content))
	if err != nil {
		return "", err
	}

	signature, err := signature.Sign(tokenBody, key)
	return base64.StdEncoding.EncodeToString(signature), err
}

func getHostMetadata() (metadata.Host, error) {
	client, err := metadata.NewClientAndWait(metadataURL)
	if err != nil {
		return metadata.Host{}, err
	}

	return client.GetSelfHost()
}

func writeToken(token string, options map[string]interface{}) error {
	devPath, ok := options["device"].(string)
	if !ok {
		return fmt.Errorf("could not find key: %s in options", "device")
	}
	values, err := getDevValues(devPath)
	if err != nil {
		return err
	}
	fullPath := path.Join(values.Get("device"), "token")

	return ioutil.WriteFile(fullPath, []byte(token)[:len(token)], os.FileMode(0644))
}

func newDeviceString(device string) string {
	val := &url.Values{}
	val.Set("device", device)
	return val.Encode()
}

func getDevValues(dev string) (url.Values, error) {
	retValues, err := url.ParseQuery(dev)
	cleanedString := strings.Replace(retValues.Get("device"), "%2F", "/", -1)
	retValues.Set("device", cleanedString)
	return retValues, err
}
