package server

import (
	"github.com/rancher/go-rancher/client"
)

type errObj struct {
	client.Resource
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

type VaultTokenInput struct {
	client.Resource
	Policies string `json:"policies"`
}

type VaultIntermediateTokenResponse struct {
	client.Resource
	Token    string `json:"token"`
	Accessor string `json:"accessor"`
}
