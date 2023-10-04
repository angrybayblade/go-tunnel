package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"github.com/google/uuid"
)

type InMemory struct {
	KeyPair *KeyPair
	Store   map[string]int
	Count   int
}

func (im *InMemory) GenerateKey() string {
	im.Count += 1
	key := Sha256([]byte(uuid.New().String()))
	im.Store[key] = im.Count
	return key
}

func (im *InMemory) IsValidAuthToken(token string) bool {
	return im.Store[token] > 0
}

func (im *InMemory) IsValidRequest(key []byte, msg string) bool {
	requestedMsg, err := im.KeyPair.Decrypt(key)
	if err != nil {
		return false
	}
	return string(requestedMsg) == msg
}

func Sha256(data []byte) string {
	s := sha256.New()
	s.Write(data)
	return strings.ToLower(base64.URLEncoding.EncodeToString(s.Sum(nil))[:KeyLen])
}
