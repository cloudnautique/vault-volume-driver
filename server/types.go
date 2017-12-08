package server

import (
	"github.com/rancher/go-rancher/client"
	"strings"
	"time"
)

type errObj struct {
	client.Resource
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

type VaultTokenInput struct {
	client.Resource
	Policies  string `json:"policies"`
	HostUUID  string `json:"hostUUID"`
	TimeStamp string `json:"timestamp"`
}

type VaultIntermediateTokenResponse struct {
	client.Resource
	Token    string `json:"token"`
	Accessor string `json:"accessor"`
}

type VaultTokenExpireInput struct {
	client.Resource
	Accessor  string `json:"accessor"`
	TimeStamp string `json:"timestamp"`
	HostUUID  string `json:"hostUUID"`
}

func (vti *VaultTokenInput) Prepare() []byte {
	return []byte(strings.Join([]string{vti.Policies, vti.HostUUID, vti.TimeStamp}, ","))
}

func (vte *VaultTokenExpireInput) Prepare() []byte {
	return []byte(strings.Join([]string{vte.Accessor, vte.TimeStamp, vte.HostUUID}, ","))
}

func (vti *VaultTokenInput) SetTimeStamp() {
	vti.TimeStamp = setTimeStamp()
}

func (vte *VaultTokenExpireInput) SetTimeStamp() {
	vte.TimeStamp = setTimeStamp()
}

func (vti *VaultTokenInput) GetTimeStamp() (*time.Time, error) {
	return getTimeStampTime(vti.TimeStamp)
}

func (vte *VaultTokenExpireInput) GetTimeStamp() (*time.Time, error) {
	return getTimeStampTime(vte.TimeStamp)
}

func setTimeStamp() string {
	timeByte, _ := time.Now().UTC().MarshalText()
	return string(timeByte)
}

func getTimeStampTime(ts string) (*time.Time, error) {
	t := &time.Time{}
	err := t.UnmarshalText([]byte(ts))
	return t, err
}
