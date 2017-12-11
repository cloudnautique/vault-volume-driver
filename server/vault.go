package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/hashicorp/vault/api"
)

const (
	tokenRefreshTime = 300 //will be converted to time seconds
)

type VaultClient struct {
	url                 string
	token               string
	role                string
	vClient             *api.Client
	healthy             bool
	creationTTL         int
	instanceTokenConfig *instanceTokenConfig
}

type instanceTokenConfig struct {
	TTL             string
	Renewable       bool
	IntermediateTTL string
}

type IntermediateToken struct {
	Accessor string
	Token    string
}

func NewVaultClient(url, token, role string) (*VaultClient, error) {
	client := &VaultClient{
		url:   url,
		token: token,
		role:  role,
	}

	err := client.vaultClient()
	if err != nil {
		return client, err
	}

	if err := client.InspectIssuingTokenForConfig(); err != nil {
		return client, err
	}

	if err := client.StartTokenRefresh(); err != nil {
		return client, err
	}

	return client, nil
}

func (vc *VaultClient) NewWrappedVaultToken(policies []string) (*IntermediateToken, error) {
	token := &IntermediateToken{}

	tokenCreateRequest := &api.TokenCreateRequest{
		Policies:  policies,
		TTL:       vc.instanceTokenConfig.TTL,
		Renewable: &vc.instanceTokenConfig.Renewable,
	}

	sec, err := vc.createVaultToken(tokenCreateRequest)
	if err != nil {
		return token, err
	}

	token.Accessor = sec.WrapInfo.Accessor
	token.Token = sec.WrapInfo.Token

	return token, nil
}

func (vc *VaultClient) RevokeToken(accessor string) error {
	return vc.vClient.Auth().Token().RevokeAccessor(accessor)
}

func (vc *VaultClient) createVaultToken(tcr *api.TokenCreateRequest) (*api.Secret, error) {
	header := http.Header{}
	header.Add("X-Vault-Wrap-TTL", vc.instanceTokenConfig.IntermediateTTL)
	vc.vClient.SetHeaders(header)

	return vc.vClient.Auth().Token().CreateWithRole(tcr, vc.role)
}

func (vc *VaultClient) vaultClient() error {
	config := api.DefaultConfig()
	config.Address = vc.url

	client, err := api.NewClient(config)
	if err != nil {
		return err
	}

	client.SetToken(vc.token)
	vc.vClient = client

	return nil
}

func (vc *VaultClient) InspectIssuingTokenForConfig() error {
	selfIntrospectedToken, err := vc.vClient.Auth().Token().LookupSelf()
	if err != nil {
		return err
	}

	if !selfIntrospectedToken.Data["renewable"].(bool) {
		return fmt.Errorf("issuing token is not renewable")
	}

	vc.creationTTL, err = getIntFromJSONInterface(selfIntrospectedToken.Data["creation_ttl"])
	if err != nil {
		return err
	}

	tokenConfig := &instanceTokenConfig{
		TTL:             "5m",
		Renewable:       true,
		IntermediateTTL: "5m",
	}

	vc.instanceTokenConfig = tokenConfig

	logrus.Debugf("token: %#v", selfIntrospectedToken.Data)
	if instanceTTL, ok := selfIntrospectedToken.Data["meta"].(map[string]interface{})["ttl"].(string); ok {
		vc.instanceTokenConfig.TTL = instanceTTL
	}

	if intermediate, ok := selfIntrospectedToken.Data["meta"].(map[string]interface{})["intermediateTTL"].(string); ok {
		vc.instanceTokenConfig.IntermediateTTL = intermediate
	}

	if renewable, ok := selfIntrospectedToken.Data["meta"].(map[string]interface{})["renewable"].(string); ok {
		if renewable == "false" {
			vc.instanceTokenConfig.Renewable = false
		}
	}

	logrus.Debugf("TOKEN CONFIG: %#v", vc.instanceTokenConfig)

	return nil
}

func (vc *VaultClient) StartTokenRefresh() error {
	_, err := vc.vClient.Auth().Token().RenewSelf(vc.creationTTL)
	if err != nil {
		vc.healthy = false
		logrus.Errorf("could not renew token: %s", err)
	}

	// This should be a long TTL so that there is opportunity to refresh, and recover if Vault
	// Goes down.
	if vc.creationTTL <= 300 {
		return fmt.Errorf("token ttl needs to be greater then 5 minutes. Ideally, this should be 1-12 hours")
	}

	go vc.refresher()

	return nil
}

func (vc *VaultClient) Healthy() bool {
	return vc.healthy
}

func getIntFromJSONInterface(value interface{}) (int, error) {
	var val int

	switch v := value.(type) {
	case float64:
		val = int(v)
	case json.Number:
		intermediate, err := v.Float64()
		if err != nil {
			return val, err
		}
		val = int(intermediate)
	}

	return val, nil
}

func (vc *VaultClient) refresher() {
	renewChannel := make(chan bool)

	go scheduleTimer(tokenRefreshTime, renewChannel)

	for {
		select {
		case <-renewChannel:
			logrus.Debug("renewing token")
			_, err := vc.vClient.Auth().Token().RenewSelf(vc.creationTTL)
			if err != nil {
				vc.healthy = false
				break
			}

			vc.healthy = true
		}
	}
}

func scheduleTimer(duration int, notify chan bool) {
	logrus.Debugf("scheduling refresh timer for: %d", duration)
	for {
		time.Sleep(time.Duration(duration) * time.Second)
		notify <- true
	}
}
