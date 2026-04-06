package auth

import (
	"net/http"
	"time"
)

type Authenticator struct {
	users  UserRepository
	tokens TokenManager
	now    func() time.Time
}

func NewAuthenticator(users UserRepository, tokens TokenManager, now func() time.Time) *Authenticator {
	if now == nil {
		now = time.Now
	}

	return &Authenticator{
		users:  users,
		tokens: tokens,
		now:    now,
	}
}

func (a *Authenticator) AuthenticateRequest(r *http.Request) (User, error) {
	token, err := ParseBearerToken(r.Header.Get("Authorization"))
	if err != nil {
		return User{}, ErrInvalidCredentials
	}

	claims, err := a.tokens.ParseAccessToken(token, a.now().UTC())
	if err != nil {
		return User{}, ErrInvalidCredentials
	}

	userID, err := MustSubjectID(claims.Subject)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}

	user, err := a.users.FindByID(r.Context(), userID)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}

	if user.Status != UserStatusActive {
		return User{}, ErrUserDisabled
	}

	return sanitizeUser(user), nil
}

type ProtectedHandler func(http.ResponseWriter, *http.Request, User)

func (a *Authenticator) Require(next ProtectedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := a.AuthenticateRequest(r)
		if err != nil {
			writeMappedAuthError(w, err)
			return
		}

		next(w, r, user)
	}
}
