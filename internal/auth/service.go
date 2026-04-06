package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
)

const minimumPasswordLength = 8

type Service struct {
	users         UserRepository
	refreshTokens RefreshTokenRepository
	passwords     PasswordHasher
	tokens        TokenManager
	now           func() time.Time
}

func NewService(
	users UserRepository,
	refreshTokens RefreshTokenRepository,
	passwords PasswordHasher,
	tokens TokenManager,
	now func() time.Time,
) *Service {
	if now == nil {
		now = time.Now
	}

	return &Service{
		users:         users,
		refreshTokens: refreshTokens,
		passwords:     passwords,
		tokens:        tokens,
		now:           now,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (User, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return User{}, err
	}

	if err := validatePassword(input.Password); err != nil {
		return User{}, err
	}

	passwordHash, err := s.passwords.Hash(input.Password)
	if err != nil {
		return User{}, err
	}

	user, err := s.users.Create(ctx, CreateUserParams{
		Email:        email,
		PasswordHash: passwordHash,
		Status:       UserStatusActive,
	})
	if err != nil {
		return User{}, err
	}

	return user, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, err
	}

	if user.Status != UserStatusActive {
		return AuthResult{}, ErrUserDisabled
	}

	if !s.passwords.Verify(input.Password, user.PasswordHash) {
		return AuthResult{}, ErrInvalidCredentials
	}

	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		User:   sanitizeUser(user),
		Tokens: tokens,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, input RefreshInput) (AuthResult, error) {
	if strings.TrimSpace(input.RefreshToken) == "" {
		return AuthResult{}, ErrInvalidRefreshToken
	}

	now := s.now().UTC()
	tokenHash := s.tokens.HashRefreshToken(input.RefreshToken)
	storedToken, err := s.refreshTokens.FindActiveByHash(ctx, tokenHash, now)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AuthResult{}, ErrInvalidRefreshToken
		}
		return AuthResult{}, err
	}

	user, err := s.users.FindByID(ctx, storedToken.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AuthResult{}, ErrInvalidRefreshToken
		}
		return AuthResult{}, err
	}

	if user.Status != UserStatusActive {
		return AuthResult{}, ErrUserDisabled
	}

	if err := s.refreshTokens.RevokeByID(ctx, storedToken.ID, now); err != nil {
		return AuthResult{}, err
	}

	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		User:   sanitizeUser(user),
		Tokens: tokens,
	}, nil
}

func (s *Service) Logout(ctx context.Context, input LogoutInput) error {
	if strings.TrimSpace(input.RefreshToken) == "" {
		return ErrInvalidRefreshToken
	}

	now := s.now().UTC()
	tokenHash := s.tokens.HashRefreshToken(input.RefreshToken)
	storedToken, err := s.refreshTokens.FindActiveByHash(ctx, tokenHash, now)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}

	return s.refreshTokens.RevokeByID(ctx, storedToken.ID, now)
}

func (s *Service) issueTokenPair(ctx context.Context, user User) (TokenPair, error) {
	now := s.now().UTC()

	accessToken, accessExpiresAt, err := s.tokens.IssueAccessToken(user, now)
	if err != nil {
		return TokenPair{}, err
	}

	refreshToken, refreshExpiresAt, err := s.tokens.GenerateRefreshToken(now)
	if err != nil {
		return TokenPair{}, err
	}

	if _, err := s.refreshTokens.Create(ctx, CreateRefreshTokenParams{
		UserID:    user.ID,
		TokenHash: s.tokens.HashRefreshToken(refreshToken),
		ExpiresAt: refreshExpiresAt,
	}); err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessExpiresAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshExpiresAt,
	}, nil
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return "", ErrInvalidEmail
	}

	if _, err := mail.ParseAddress(normalized); err != nil {
		return "", ErrInvalidEmail
	}

	return normalized, nil
}

func validatePassword(password string) error {
	if len(password) < minimumPasswordLength {
		return ErrWeakPassword
	}

	return nil
}

func sanitizeUser(user User) User {
	user.PasswordHash = ""
	return user
}
