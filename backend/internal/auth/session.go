package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

type SessionToken struct {
	Plaintext string
	Hash      []byte
}

func GenerateSessionToken() (SessionToken, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return SessionToken{}, err
	}
	hash := sha256.Sum256(raw)
	return SessionToken{Plaintext: base64.RawURLEncoding.EncodeToString(raw), Hash: hash[:]}, nil
}

func HashSessionToken(plaintext string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(plaintext)
	if err != nil || len(raw) != 32 {
		return nil, errors.New("invalid session token")
	}
	hash := sha256.Sum256(raw)
	return hash[:], nil
}
