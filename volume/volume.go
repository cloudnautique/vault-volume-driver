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

	"github.com/Sirupsen/logrus"
	"github.com/moby/moby/pkg/mount"
	"github.com/rancher/go-rancher-metadata/metadata"
	"github.com/rancher/secrets-api/pkg/rsautils"
	"github.com/rancher/secrets-bridge-v2/server"
	"github.com/rancher/secrets-bridge-v2/signature"
)

const (
	volRoot        = "/var/lib/rancher/volumes/secrets-bridge-v2"
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
		logrus.Debugf("volume path is: %s", volPath)

		resp["device"] = newDeviceString(volPath)
		return resp, nil
	}

	return resp, fmt.Errorf("no name was passed to driver")
}

func (v *FlexVol) Delete(options map[string]interface{}) error {
	logrus.Infof("%#v", options)
	if device, ok := options["device"].(string); ok {
		return v.Detach(device)
	}
	return nil
}

func (v *FlexVol) Attach(options map[string]interface{}) (string, error) {
	name, ok := options["name"].(string)
	if !ok {
		return "", fmt.Errorf("name not set for volume")
	}

	dev, ok := options["device"].(string)
	if !ok {
		return dev, fmt.Errorf("could not find device key in options")
	}

	devValues, err := getDeviceValues(dev)
	if err != nil {
		return dev, err
	}

	// Revoke if there was a previous accessor on the volume
	accessor := devValues.Get("accessor")
	if accessor != "" {
		makeTokenRevokeRequest(accessor)
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
		Policies:   policies,
		HostUUID:   host.UUID,
		VolumeName: name,
	}

	token, err := makeTokenRequest(req)
	if err != nil {
		return dev, err
	}

	err = createTmpfs(devValues.Get("device"), options)
	if err != nil {
		return dev, err
	}

	err = writeToken(token.EncryptedToken, options)
	if err != nil {
		logrus.Errorf("failed to write token: %s to volume. calling revoke.", err)
		makeTokenRevokeRequest(token.Accessor)
		return dev, err
	}

	devValues.Set("accessor", token.Accessor)

	err = writeAccessor(token.Accessor, devValues.Get("device"))
	return devValues.Encode(), err
}

func (v *FlexVol) Detach(device string) error {
	logrus.Infof("Device: %s", device)

	values, err := getDeviceValues(device)
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

	return os.RemoveAll(values.Get("device"))
}

func (v *FlexVol) Mount(dir string, device string, params map[string]interface{}) error {
	logrus.Debugf("Device: %s, dir: %s, params: %#v", device, dir, params)
	values, err := getDeviceValues(device)
	if err != nil {
		return err
	}
	return mount.Mount(values.Get("device"), dir, "none", "bind,rw")
}

func (v *FlexVol) Unmount(dir string) error {
	logrus.Debugf("Dir: %s", dir)

	accessorFile := path.Join(dir, ".accessor")
	if _, err := os.Stat(accessorFile); err == nil {
		accessorBytes, err := ioutil.ReadFile(accessorFile)
		if err != nil {
			return err
		}

		if err = makeTokenRevokeRequest(string(accessorBytes)); err != nil {
			return err
		}
	}

	return mount.Unmount(dir)
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
		Accessor: accessor,
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

	vteiJSON, err := json.Marshal(vtei)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", vaultTokenServerURL, bytes.NewBuffer(vteiJSON))
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

	tokenJSON, err := json.Marshal(tokenBody)
	if err != nil {
		return tokenResp, err
	}

	req, err := http.NewRequest("POST", vaultTokenServerURL, bytes.NewBuffer(tokenJSON))
	req.Header.Set(server.SignatureHeaderString, signature)
	if err != nil {
		return tokenResp, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return tokenResp, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return tokenResp, fmt.Errorf("received status code: %d msg: %s", resp.StatusCode, body)
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
	values, err := getDeviceValues(devPath)
	if err != nil {
		return err
	}
	fullPath := path.Join(values.Get("device"), "token")

	decryptor, err := rsautils.NewRSADecryptorKeyFromFile(privateKeyFile)
	if err != nil {
		return err
	}

	tokenBytes, err := decryptor.Decrypt(token)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(fullPath, tokenBytes, os.FileMode(0644))
}

func writeAccessor(accessor, devPath string) error {
	fullPath := path.Join(devPath, ".accessor")
	return ioutil.WriteFile(fullPath, []byte(accessor), os.FileMode(0644))
}

func newDeviceString(device string) string {
	val := &url.Values{}
	val.Set("device", device)
	return val.Encode()
}

func getDeviceValues(dev string) (url.Values, error) {
	retValues, err := url.ParseQuery(dev)
	cleanedString := strings.Replace(retValues.Get("device"), "%2F", "/", -1)
	retValues.Set("device", cleanedString)
	return retValues, err
}
