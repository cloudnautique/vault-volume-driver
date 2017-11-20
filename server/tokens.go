package server

import (
	"github.com/rancher/go-rancher/client"
)

// NewVaultTokenResponse returns a VaultIntermedateTokenResponse object
func NewVaultTokenResponse(intermediateToken *IntermediateToken) *VaultIntermediateTokenResponse {
	return &VaultIntermediateTokenResponse{
		Resource: client.Resource{
			Type: "vaultIntermediateToken",
		},
		Accessor: intermediateToken.Accessor,
		Token:    intermediateToken.Token,
	}
}
