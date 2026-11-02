package tokens

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

func Random() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(value), nil
}

func Hash(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}
