package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

const KeyLen int = 43

type KeyPair struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

func (kp *KeyPair) Encrypt(msg []byte) ([]byte, error) {
	return rsa.EncryptOAEP(sha1.New(), rand.Reader, kp.PublicKey, msg, nil)
}

func (kp *KeyPair) Decrypt(msg []byte) ([]byte, error) {
	return kp.PrivateKey.Decrypt(nil, msg, &rsa.OAEPOptions{Hash: crypto.SHA1})
}

func (kp *KeyPair) DumpPublicKey() ([]byte, error) {
	pubASN1, err := x509.MarshalPKIXPublicKey(kp.PublicKey)
	if err != nil {
		return []byte(""), err
	}
	pubBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: pubASN1,
	})
	return pubBytes, nil
}

func (kp *KeyPair) LoadPublicKey(pubBytes []byte) error {
	block, _ := pem.Decode(pubBytes)
	if block == nil {
		return errors.New("Failed to parse PEM block containing the key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	switch pub := pub.(type) {
	case *rsa.PublicKey:
		kp.PublicKey = pub
		return nil
	default:
		break // fall through
	}
	return errors.New("Key type is not RSA")
}

func GenerateKeyPair() (*KeyPair, error) {
	kp := &KeyPair{}
	privateKey, err := rsa.GenerateKey(rand.Reader, 340)
	if err != nil {
		return kp, err
	}
	kp.PrivateKey = privateKey
	kp.PublicKey = &privateKey.PublicKey
	return kp, nil
}
