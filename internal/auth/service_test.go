package auth

import (
	"context"
	"testing"
	"time"
)

func TestServiceRegisterLoginRefreshLogout(t *testing.T) {
	now := time.Unix(1712360000, 0).UTC()
	service := NewService(
		NewInMemoryUserRepository(),
		NewInMemoryRefreshTokenRepository(),
		NewPasswordHasher(1000, 32, 16),
		NewTokenManager("access-secret", "refresh-secret", 15*time.Minute, 24*time.Hour),
		func() time.Time { return now },
	)

	user, err := service.Register(context.Background(), RegisterInput{
		Email:    "User@example.com",
		Password: "super-secret-password",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if user.Email != "user@example.com" {
		t.Fatalf("Register() email = %q, want normalized email", user.Email)
	}

	login, err := service.Login(context.Background(), LoginInput{
		Email:    "user@example.com",
		Password: "super-secret-password",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if login.Tokens.AccessToken == "" || login.Tokens.RefreshToken == "" {
		t.Fatalf("Login() returned empty tokens")
	}

	refreshed, err := service.Refresh(context.Background(), RefreshInput{
		RefreshToken: login.Tokens.RefreshToken,
	})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if refreshed.Tokens.RefreshToken == login.Tokens.RefreshToken {
		t.Fatalf("Refresh() did not rotate refresh token")
	}

	if err := service.Logout(context.Background(), LogoutInput{
		RefreshToken: refreshed.Tokens.RefreshToken,
	}); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	if _, err := service.Refresh(context.Background(), RefreshInput{
		RefreshToken: refreshed.Tokens.RefreshToken,
	}); err == nil {
		t.Fatalf("Refresh() with revoked token = nil error, want error")
	}
}

func TestServiceRejectsDuplicateEmail(t *testing.T) {
	service := NewService(
		NewInMemoryUserRepository(),
		NewInMemoryRefreshTokenRepository(),
		NewPasswordHasher(1000, 32, 16),
		NewTokenManager("access-secret", "refresh-secret", 15*time.Minute, 24*time.Hour),
		time.Now,
	)

	_, _ = service.Register(context.Background(), RegisterInput{
		Email:    "user@example.com",
		Password: "super-secret-password",
	})

	_, err := service.Register(context.Background(), RegisterInput{
		Email:    "user@example.com",
		Password: "another-secret-password",
	})
	if err != ErrEmailAlreadyExists {
		t.Fatalf("Register() duplicate error = %v, want %v", err, ErrEmailAlreadyExists)
	}
}

func TestServiceRejectsInvalidRegistrationInput(t *testing.T) {
	service := NewService(
		NewInMemoryUserRepository(),
		NewInMemoryRefreshTokenRepository(),
		NewPasswordHasher(1000, 32, 16),
		NewTokenManager("access-secret", "refresh-secret", 15*time.Minute, 24*time.Hour),
		time.Now,
	)

	tests := []struct {
		name  string
		input RegisterInput
		want  error
	}{
		{
			name:  "invalid email",
			input: RegisterInput{Email: "not-an-email", Password: "super-secret-password"},
			want:  ErrInvalidEmail,
		},
		{
			name:  "weak password",
			input: RegisterInput{Email: "user@example.com", Password: "short"},
			want:  ErrWeakPassword,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.Register(context.Background(), tc.input)
			if err != tc.want {
				t.Fatalf("Register() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestServiceLoginAndRefreshRejectInvalidCredentials(t *testing.T) {
	service := NewService(
		NewInMemoryUserRepository(),
		NewInMemoryRefreshTokenRepository(),
		NewPasswordHasher(1000, 32, 16),
		NewTokenManager("access-secret", "refresh-secret", 15*time.Minute, 24*time.Hour),
		time.Now,
	)

	if _, err := service.Register(context.Background(), RegisterInput{
		Email:    "user@example.com",
		Password: "super-secret-password",
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if _, err := service.Login(context.Background(), LoginInput{
		Email:    "user@example.com",
		Password: "wrong-password",
	}); err != ErrInvalidCredentials {
		t.Fatalf("Login() error = %v, want %v", err, ErrInvalidCredentials)
	}

	if _, err := service.Refresh(context.Background(), RefreshInput{}); err != ErrInvalidRefreshToken {
		t.Fatalf("Refresh() error = %v, want %v", err, ErrInvalidRefreshToken)
	}
}
