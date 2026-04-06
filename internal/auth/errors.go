package auth

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrEmailAlreadyExists  = errors.New("email already exists")
	ErrInvalidEmail        = errors.New("invalid email")
	ErrWeakPassword        = errors.New("weak password")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrUserDisabled        = errors.New("user disabled")
)
