package signature

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"

	"github.com/Sirupsen/logrus"
)

type Message interface {
	Prepare() []byte
}

func Sign(message Message, privateKey *rsa.PrivateKey) ([]byte, error) {
	rand := rand.Reader

	hashed := sha256.Sum256(message.Prepare())

	return rsa.SignPKCS1v15(rand, privateKey, crypto.SHA256, hashed[:])
}

func Verify(signature []byte, message Message, publicKey *rsa.PublicKey) (bool, error) {
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
	block, val := pem.Decode([]byte(key))
	if block == nil {
		logrus.Debugf(string(val))
		return nil, errors.New("could not decode public key block")
	}

	logrus.Debugf("Public Key Block Type: %s", block.Type)

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return pub.(*rsa.PublicKey), nil
}
