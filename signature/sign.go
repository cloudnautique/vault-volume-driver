package signature

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/secrets-api/pkg/rsautils"
)

const (
	TimeWindow = 5 * time.Minute
)

type Message interface {
	Prepare() []byte
	GetTimeStamp() (*time.Time, error)
	SetTimeStamp()
}

func Sign(message Message, privateKey *rsa.PrivateKey) ([]byte, error) {
	rand := rand.Reader
	message.SetTimeStamp()

	hashed := sha256.Sum256(message.Prepare())

	return rsa.SignPKCS1v15(rand, privateKey, crypto.SHA256, hashed[:])
}

func Verify(signature []byte, message Message, publicKey *rsa.PublicKey) (bool, error) {
	time, err := message.GetTimeStamp()
	if err != nil {
		return false, err
	}

	if timeWindowExpired(time) {
		return false, err
	}

	hashed := sha256.Sum256(message.Prepare())

	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], signature); err != nil {
		return false, err
	}
	return true, nil
}

func LoadPrivateKeyFromString(key string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(key))
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func LoadRSAPublicKey(key string) (*rsa.PublicKey, error) {
	rsakey, err := rsautils.PublicKeyFromString(key)
	return rsakey.PublicKey, err
}

func timeWindowExpired(ts *time.Time) bool {
	duration := time.Since(*ts)
	logrus.Debugf("duration: %s", duration)
	if duration > TimeWindow {
		return true
	}
	return false
}
