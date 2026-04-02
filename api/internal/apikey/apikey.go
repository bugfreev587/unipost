package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	// Key size in bytes before base58 encoding
	keySize = 32

	PrefixLive = "up_live_"
	PrefixTest = "up_test_"
)

// base58 alphabet (Bitcoin style, no 0/O/I/l)
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// Generate creates a new API key and returns the full plaintext key, prefix, and SHA-256 hash.
// The plaintext key should only be shown to the user once.
func Generate(environment string) (plaintext, prefix, hash string, err error) {
	raw := make([]byte, keySize)
	if _, err := rand.Read(raw); err != nil {
		return "", "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	encoded := base58Encode(raw)

	var keyPrefix string
	switch environment {
	case "test":
		keyPrefix = PrefixTest
	default:
		keyPrefix = PrefixLive
	}

	plaintext = keyPrefix + encoded
	prefix = keyPrefix + encoded[:8]
	hash = Hash(plaintext)

	return plaintext, prefix, hash, nil
}

// Hash returns the hex-encoded SHA-256 hash of the given key.
func Hash(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func base58Encode(input []byte) string {
	if len(input) == 0 {
		return ""
	}

	// Convert byte slice to a big number, then repeatedly divide by 58
	// Simple implementation without big.Int dependency
	intBytes := make([]byte, len(input))
	copy(intBytes, input)

	var result []byte
	for len(intBytes) > 0 {
		var remainder int
		var newBytes []byte
		for _, b := range intBytes {
			value := remainder*256 + int(b)
			digit := value / 58
			remainder = value % 58
			if len(newBytes) > 0 || digit > 0 {
				newBytes = append(newBytes, byte(digit))
			}
		}
		result = append(result, base58Alphabet[remainder])
		intBytes = newBytes
	}

	// Reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}
