package server

import (
	"net/http"

	"github.com/hashicorp/vault/api"
)

type VaultClient struct {
	url     string
	token   string
	role    string
	vClient *api.Client
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

	return client, nil
}

func (vc *VaultClient) NewWrappedVaultToken(policies []string) (*IntermediateToken, error) {
	token := &IntermediateToken{}

	tokenCreateRequest := &api.TokenCreateRequest{
		Policies: policies,
		TTL:      "5m",
		NumUses:  1,
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
	header.Add("X-Vault-Wrap-TTL", "5m")
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
