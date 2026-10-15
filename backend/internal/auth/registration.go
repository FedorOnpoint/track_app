package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidInvitation       = errors.New("invitation is invalid, expired or already used")
	ErrLoginUnavailable        = errors.New("login is unavailable")
	ErrRegistrationRateLimited = errors.New("too many registration attempts")
)

type RegistrationStore interface {
	TakeInvitationAttempt(context.Context, []byte, time.Time) error
	RegisterWithInvitation(context.Context, User, Session, []byte, time.Time) error
}

type RegistrationService struct {
	store     RegistrationStore
	passwords PasswordHasher
	ttl       time.Duration
}

func NewRegistrationService(store RegistrationStore, passwords PasswordHasher, ttl time.Duration) *RegistrationService {
	return &RegistrationService{store: store, passwords: passwords, ttl: ttl}
}

func (s *RegistrationService) Register(ctx context.Context, login, password, code string, now time.Time) (LoginResult, error) {
	if !loginPattern.MatchString(login) {
		return LoginResult{}, ErrInvalidLogin
	}
	if len(password) < 12 || len(password) > 1024 {
		return LoginResult{}, ErrInvalidPassword
	}
	inviteHash, err := HashSessionToken(code)
	if err != nil {
		return LoginResult{}, ErrInvalidInvitation
	}
	// The shared database quota is checked before the expensive password hash.
	if err := s.store.TakeInvitationAttempt(ctx, inviteHash, now); err != nil {
		return LoginResult{}, err
	}
	passwordHash, err := s.passwords.Hash(password)
	if err != nil {
		return LoginResult{}, err
	}
	token, err := GenerateSessionToken()
	if err != nil {
		return LoginResult{}, err
	}
	user := User{ID: uuid.New(), Login: login, PasswordHash: passwordHash}
	session := Session{ID: uuid.New(), UserID: user.ID, TokenHash: token.Hash, ExpiresAt: now.Add(s.ttl)}
	if err := s.store.RegisterWithInvitation(ctx, user, session, inviteHash, now); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{User: user, Token: token.Plaintext}, nil
}
