package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/moby/moby/pkg/mount"
	"github.com/rancher/go-rancher-metadata/metadata"
	"github.com/rancher/vault-volume-driver/server"
	"github.com/rancher/vault-volume-driver/signature"
	"net/url"
	"strings"
)

const (
	volRoot             = "/var/lib/rancher/volumes/vault-volume-driver"
	metadataURL         = "http://169.254.169.250/2016-07-29"
	vaultTokenServerURL = "http://10.20.0.4:8080/v1-vault-driver/tokens"
	privateKeyFile      = "/var/lib/rancher/etc/ssl/host.key"
)

type FlexVol struct{}

func (v *FlexVol) Init() error { return nil }

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
		TimeStamp: string(time.Now().UTC().String()),
	}

	token, err := makeTokenRequest(req)
	if err != nil {
		return dev, err
	}

	err = writeToken(token.Token, options)
	if err != nil {
		return dev, err
	}

	devValues.Add("accessor", token.Accessor)

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

	//TODO: call revoke endpoint

	return os.RemoveAll(device)
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
	return os.RemoveAll(dir)
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

	jsonDecoder := json.NewDecoder(resp.Body)
	err = jsonDecoder.Decode(tokenResp)

	return tokenResp, err
}

func getSignature(tokenBody *server.VaultTokenInput) (string, error) {
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

//func updateRancherVolumeOptions(options map[string]interface{}) error {
//	rClient, err := rancher.NewRancherClientFromContainerEnv()
//	if err != nil {
//		return err
//	}
//
//	volumes, err := rClient.Volume.List(&client.ListOpts{
//		Filters: map[string]interface{}{
//			"name": options["name"].(string),
//		},
//	})
//	if err != nil {
//		return err
//	}
//
//	if len(volumes.Data) != 1 {
//		return fmt.Errorf("problem finding volume named: %s expected 1 instance got: %d", options["name"].(string), len(volumes.Data))
//	}
//
//	volume := volumes.Data[0]
//	_, err = rClient.Volume.Update(&volume, &client.Volume{
//		Name:            volume.Name,
//		Driver:          volume.Driver,
//		StorageDriverId: volume.StorageDriverId,
//		DriverOpts:      options,
//		HostId:          volume.HostId,
//	})
//	return err
//}

func newDeviceString(device string) string {
	val := &url.Values{}
	val.Set("device", device)
	return val.Encode()
}

func getDevValues(dev string) (url.Values, error) {
	retValues, err := url.ParseQuery(dev)
	cleanedString := strings.Replace(retValues.Get("device"), "%2F", "/", -1)
	logrus.Infof("CleanedString: %s", cleanedString)
	retValues.Set("device", cleanedString)
	return retValues, err
}
