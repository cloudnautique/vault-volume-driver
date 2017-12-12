package rancher

import (
	"fmt"

	"github.com/rancher/go-rancher/v2"
	"os"
)

func NewRancherClient(url, accessKey, secretKey string) (*client.RancherClient, error) {
	opts := &client.ClientOpts{
		Url:       url,
		AccessKey: accessKey,
		SecretKey: secretKey,
	}
	return client.NewRancherClient(opts)
}

func NewRancherClientFromContainerEnv() (*client.RancherClient, error) {
	opts := &client.ClientOpts{
		Url:       os.Getenv("CATTLE_URL"),
		AccessKey: os.Getenv("CATTLE_ACCESS_KEY"),
		SecretKey: os.Getenv("CATTLE_SECRET_KEY"),
	}
	return client.NewRancherClient(opts)
}

func GetRancherHostPublicKey(rClient *client.RancherClient, hostUUID string) (string, error) {
	// TODO: add a cache here possibly use hashicorp/lru
	hosts, err := rClient.Host.List(&client.ListOpts{
		Filters: map[string]interface{}{
			"uuid": hostUUID,
		},
	})
	if err != nil {
		return "", err
	}

	if len(hosts.Data) > 0 {
		return hosts.Data[0].Info.(map[string]interface{})["hostKey"].(map[string]interface{})["data"].(string), nil
	}

	return "", fmt.Errorf("host: %s not found", hostUUID)
}
