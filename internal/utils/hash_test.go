// internal/utils/hash_test.go
package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashIdentifierWithKey(t *testing.T) {
	identifier := "user@example.com"
	secretKey := "secret-key"

	result := HashIdentifierWithKey(identifier, secretKey)

	require.NotEmpty(t, result)
	assert.Len(t, result, 64)
}

func TestHashIdentifierWithKey_IsDeterministic(t *testing.T) {
	identifier := "user@example.com"
	secretKey := "secret-key"

	first := HashIdentifierWithKey(identifier, secretKey)
	second := HashIdentifierWithKey(identifier, secretKey)

	assert.Equal(t, first, second)
}

func TestHashIdentifierWithKey_DifferentIdentifiers(t *testing.T) {
	secretKey := "secret-key"

	first := HashIdentifierWithKey("user1@example.com", secretKey)
	second := HashIdentifierWithKey("user2@example.com", secretKey)

	assert.NotEqual(t, first, second)
}

func TestHashIdentifierWithKey_DifferentKeys(t *testing.T) {
	identifier := "user@example.com"

	first := HashIdentifierWithKey(identifier, "secret-key-1")
	second := HashIdentifierWithKey(identifier, "secret-key-2")

	assert.NotEqual(t, first, second)
}

func TestHashIdentifierWithKey_IsHexadecimal(t *testing.T) {
	result := HashIdentifierWithKey(
		"user@example.com",
		"secret-key",
	)

	for _, char := range result {
		assert.True(
			t,
			(char >= '0' && char <= '9') ||
				(char >= 'a' && char <= 'f'),
			"unexpected non-hex character: %q",
			char,
		)
	}
}