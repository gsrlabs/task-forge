package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// HashIdentifierWithKey hashes the identifier (email, user_id) using a secret key.
func HashIdentifierWithKey(identifier, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(identifier))
	return hex.EncodeToString(h.Sum(nil))
}