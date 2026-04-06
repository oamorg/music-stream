package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type AccessClaims struct {
	Subject string
	Email   string
	Expires time.Time
	Issued  time.Time
}

type TokenManager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

func NewTokenManager(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) TokenManager {
	return TokenManager{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

func (m TokenManager) IssueAccessToken(user User, now time.Time) (string, time.Time, error) {
	expiresAt := now.UTC().Add(m.accessTTL)

	headerJSON, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", time.Time{}, err
	}

	payloadJSON, err := json.Marshal(map[string]any{
		"sub":   strconv.FormatInt(user.ID, 10),
		"email": user.Email,
		"typ":   "access",
		"iat":   now.UTC().Unix(),
		"exp":   expiresAt.Unix(),
	})
	if err != nil {
		return "", time.Time{}, err
	}

	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := header + "." + payload
	signature := signHS256(m.accessSecret, signingInput)

	return signingInput + "." + signature, expiresAt, nil
}

func (m TokenManager) ParseAccessToken(token string, now time.Time) (AccessClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return AccessClaims{}, ErrInvalidCredentials
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSignature := signHS256(m.accessSecret, signingInput)
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSignature)) {
		return AccessClaims{}, ErrInvalidCredentials
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AccessClaims{}, ErrInvalidCredentials
	}

	var payload map[string]any
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return AccessClaims{}, ErrInvalidCredentials
	}

	tokenType, _ := payload["typ"].(string)
	if tokenType != "access" {
		return AccessClaims{}, ErrInvalidCredentials
	}

	expRaw, ok := payload["exp"].(float64)
	if !ok {
		return AccessClaims{}, ErrInvalidCredentials
	}

	iatRaw, ok := payload["iat"].(float64)
	if !ok {
		return AccessClaims{}, ErrInvalidCredentials
	}

	expiresAt := time.Unix(int64(expRaw), 0).UTC()
	if !expiresAt.After(now.UTC()) {
		return AccessClaims{}, ErrInvalidCredentials
	}

	subject, _ := payload["sub"].(string)
	email, _ := payload["email"].(string)
	if subject == "" || email == "" {
		return AccessClaims{}, ErrInvalidCredentials
	}

	return AccessClaims{
		Subject: subject,
		Email:   email,
		Expires: expiresAt,
		Issued:  time.Unix(int64(iatRaw), 0).UTC(),
	}, nil
}

func (m TokenManager) GenerateRefreshToken(now time.Time) (string, time.Time, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", time.Time{}, err
	}

	return base64.RawURLEncoding.EncodeToString(randomBytes), now.UTC().Add(m.refreshTTL), nil
}

func (m TokenManager) HashRefreshToken(token string) string {
	mac := hmac.New(sha256.New, m.refreshSecret)
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func signHS256(secret []byte, message string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func ParseBearerToken(headerValue string) (string, error) {
	if headerValue == "" {
		return "", ErrInvalidCredentials
	}

	prefix := "Bearer "
	if !strings.HasPrefix(headerValue, prefix) {
		return "", ErrInvalidCredentials
	}

	token := strings.TrimSpace(strings.TrimPrefix(headerValue, prefix))
	if token == "" {
		return "", ErrInvalidCredentials
	}

	return token, nil
}

func MustSubjectID(subject string) (int64, error) {
	id, err := strconv.ParseInt(subject, 10, 64)
	if err != nil {
		return 0, errors.New("invalid subject")
	}

	return id, nil
}

func (m TokenManager) String() string {
	return fmt.Sprintf("TokenManager{accessTTL:%s,refreshTTL:%s}", m.accessTTL, m.refreshTTL)
}
