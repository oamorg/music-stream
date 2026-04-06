package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const (
	DefaultPasswordIterations = 100000
	DefaultPasswordKeyLength  = 32
	DefaultPasswordSaltLength = 16
)

type PasswordHasher struct {
	Iterations int
	KeyLength  int
	SaltLength int
}

func NewPasswordHasher(iterations, keyLength, saltLength int) PasswordHasher {
	return PasswordHasher{
		Iterations: iterations,
		KeyLength:  keyLength,
		SaltLength: saltLength,
	}
}

func (h PasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, h.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	derived := pbkdf2SHA256([]byte(password), salt, h.Iterations, h.KeyLength)

	return fmt.Sprintf(
		"pbkdf2_sha256$%d$%s$%s",
		h.Iterations,
		base64.RawURLEncoding.EncodeToString(salt),
		base64.RawURLEncoding.EncodeToString(derived),
	), nil
}

func (h PasswordHasher) Verify(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}

	salt, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}

	expected, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}

	actual := pbkdf2SHA256([]byte(password), salt, iterations, len(expected))
	return hmac.Equal(actual, expected)
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLength int) []byte {
	hashLength := sha256.Size
	blockCount := (keyLength + hashLength - 1) / hashLength
	derived := make([]byte, 0, blockCount*hashLength)

	for block := 1; block <= blockCount; block++ {
		u := pbkdf2Block(password, salt, block)
		t := make([]byte, len(u))
		copy(t, u)

		for i := 1; i < iterations; i++ {
			u = pbkdf2PRF(password, u)
			for j := range t {
				t[j] ^= u[j]
			}
		}

		derived = append(derived, t...)
	}

	return derived[:keyLength]
}

func pbkdf2Block(password, salt []byte, block int) []byte {
	input := make([]byte, 0, len(salt)+4)
	input = append(input, salt...)
	input = append(input, byte(block>>24), byte(block>>16), byte(block>>8), byte(block))
	return pbkdf2PRF(password, input)
}

func pbkdf2PRF(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
