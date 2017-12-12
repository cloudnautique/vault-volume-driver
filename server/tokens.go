package server

import (
	"github.com/rancher/go-rancher/client"
	"github.com/rancher/secrets-api/pkg/rsautils"
)

// NewVaultTokenResponse returns a VaultIntermedateTokenResponse object
func NewVaultTokenResponse(intermediateToken *IntermediateToken, pubKey string) (*VaultIntermediateTokenResponse, error) {
	resp := &VaultIntermediateTokenResponse{
		Resource: client.Resource{
			Type: "vaultIntermediateToken",
		},
		Accessor: intermediateToken.Accessor,
	}

	pubKeyEncryptor, err := rsautils.PublicKeyFromString(pubKey)
	if err != nil {
		return resp, err
	}

	resp.EncryptedToken, err = pubKeyEncryptor.Encrypt(intermediateToken.Token)
	return resp, err
}
