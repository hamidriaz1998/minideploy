package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const Version = "0.1.0"

var keyLength = 32

func GenerateAPIKey() (string, string, error) {
	bytes := make([]byte, keyLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}
	raw := hex.EncodeToString(bytes)

	hash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("hash key: %w", err)
	}

	return raw, string(hash), nil
}


