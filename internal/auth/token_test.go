package auth

import (
	"testing"
	"time"
)

func TestTokenManagerIssueAndParseAccessToken(t *testing.T) {
	manager := NewTokenManager("access-secret", "refresh-secret", 15*time.Minute, 24*time.Hour)
	now := time.Unix(1712360000, 0).UTC()

	token, expiresAt, err := manager.IssueAccessToken(User{
		ID:    42,
		Email: "user@example.com",
	}, now)
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	claims, err := manager.ParseAccessToken(token, now.Add(1*time.Minute))
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}

	if claims.Subject != "42" {
		t.Fatalf("Subject = %q, want %q", claims.Subject, "42")
	}

	if claims.Email != "user@example.com" {
		t.Fatalf("Email = %q, want %q", claims.Email, "user@example.com")
	}

	if !claims.Expires.Equal(expiresAt) {
		t.Fatalf("Expires = %v, want %v", claims.Expires, expiresAt)
	}
}

func TestTokenManagerHashesRefreshTokenDeterministically(t *testing.T) {
	manager := NewTokenManager("access-secret", "refresh-secret", 15*time.Minute, 24*time.Hour)

	first := manager.HashRefreshToken("refresh-token")
	second := manager.HashRefreshToken("refresh-token")

	if first != second {
		t.Fatalf("HashRefreshToken() returned inconsistent hashes")
	}
}
