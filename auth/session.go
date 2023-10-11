package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"github.com/google/uuid"
)

func Sha256(data []byte) string {
	s := sha256.New()
	s.Write(data)
	return strings.ToLower(base64.URLEncoding.EncodeToString(s.Sum(nil))[:KeyLen])
}

type AuthSession interface {
	Store() map[string]int
	Count() int
	GenerateKey() string
	DeleteKey(key string)
	IsValidAuthToken(token string) bool
	IsValidRequest(key []byte, msg string) bool
}

type InMemory struct {
	KeyPair *KeyPair
	store   map[string]int
	count   int
}

func (im *InMemory) Store() map[string]int {
	return im.store
}

func (im *InMemory) Count() int {
	return im.count
}

func (im *InMemory) GenerateKey() string {
	im.count += 1
	key := Sha256([]byte(uuid.New().String()))
	im.store[key] = im.count
	return key
}

func (im *InMemory) DeleteKey(key string) {
	delete(im.store, key)
}

func (im *InMemory) IsValidAuthToken(token string) bool {
	return im.store[token] > 0
}

func (im *InMemory) IsValidRequest(key []byte, msg string) bool {
	requestedMsg, err := im.KeyPair.Decrypt(key)
	if err != nil {
		return false
	}
	return string(requestedMsg) == msg
}

type DefaultSession struct {
	store map[string]int
	count int
}

func (im *DefaultSession) Store() map[string]int {
	return im.store
}

func (im *DefaultSession) Count() int {
	return im.count
}

func (im *DefaultSession) GenerateKey() string {
	im.count += 1
	key := Sha256([]byte(uuid.New().String()))
	im.store[key] = im.count
	return key
}

func (im *DefaultSession) DeleteKey(key string) {
	delete(im.store, key)
}

func (im *DefaultSession) IsValidAuthToken(token string) bool {
	return im.store[token] > 0
}

func (im *DefaultSession) IsValidRequest(key []byte, msg string) bool {
	return true
}

func NewDefaultSession(key string) *DefaultSession {
	ds := &DefaultSession{
		store: map[string]int{
			key: 1,
		},
	}
	ds.count = 1
	return ds
}

func NewInMemorySession(keyPair *KeyPair) *InMemory {
	return &InMemory{
		KeyPair: keyPair,
		store:   make(map[string]int),
	}
}
