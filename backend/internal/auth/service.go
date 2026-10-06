package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const dummyPasswordHash = "$argon2id$v=19$m=19456,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

var ErrInvalidCredentials = errors.New("invalid credentials")

type User struct {
	ID           uuid.UUID
	Login        string
	PasswordHash string
}

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
}

type UserSession struct {
	User
	SessionID uuid.UUID
}

type LoginResult struct {
	User  User
	Token string
}

type SessionStore interface {
	FindUserByLogin(context.Context, string) (User, error)
	CreateSession(context.Context, Session) error
	FindUserByTokenHash(context.Context, []byte, time.Time) (UserSession, error)
	RevokeSession(context.Context, uuid.UUID, time.Time) error
	RevokeAllSessions(context.Context, uuid.UUID, time.Time) error
	ChangePasswordAndRotate(context.Context, uuid.UUID, uuid.UUID, string, Session, time.Time) error
}

type PasswordVerifier interface {
	Verify(password, encoded string) (bool, error)
	Hash(password string) (string, error)
}

type Service struct {
	store    SessionStore
	verifier PasswordVerifier
	ttl      time.Duration
}

func NewService(store SessionStore, verifier PasswordVerifier, ttl time.Duration) *Service {
	return &Service{store: store, verifier: verifier, ttl: ttl}
}

func (service *Service) Login(ctx context.Context, login, password string, now time.Time) (LoginResult, error) {
	user, err := service.store.FindUserByLogin(ctx, login)
	hash := dummyPasswordHash
	userFound := err == nil
	if userFound {
		hash = user.PasswordHash
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return LoginResult{}, err
	}
	matched, verifyErr := service.verifier.Verify(password, hash)
	if verifyErr != nil || !userFound || !matched {
		return LoginResult{}, ErrInvalidCredentials
	}
	token, err := GenerateSessionToken()
	if err != nil {
		return LoginResult{}, err
	}
	session := Session{ID: uuid.New(), UserID: user.ID, TokenHash: token.Hash, ExpiresAt: now.Add(service.ttl)}
	if err := service.store.CreateSession(ctx, session); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{User: user, Token: token.Plaintext}, nil
}

func (service *Service) Authenticate(ctx context.Context, plaintext string, now time.Time) (UserSession, error) {
	hash, err := HashSessionToken(plaintext)
	if err != nil {
		return UserSession{}, ErrInvalidCredentials
	}
	userSession, err := service.store.FindUserByTokenHash(ctx, hash, now)
	if err != nil {
		return UserSession{}, ErrInvalidCredentials
	}
	return userSession, nil
}

func (service *Service) Logout(ctx context.Context, plaintext string, now time.Time) error {
	userSession, err := service.Authenticate(ctx, plaintext, now)
	if err != nil {
		return nil
	}
	return service.store.RevokeSession(ctx, userSession.SessionID, now)
}

func (service *Service) LogoutAll(ctx context.Context, plaintext string, now time.Time) error {
	userSession, err := service.Authenticate(ctx, plaintext, now)
	if err != nil {
		return ErrInvalidCredentials
	}
	return service.store.RevokeAllSessions(ctx, userSession.User.ID, now)
}

func (service *Service) ChangePassword(ctx context.Context, plaintext, currentPassword, newPassword string, now time.Time) (LoginResult, error) {
	if len(newPassword) < 12 || len(newPassword) > 1024 {
		return LoginResult{}, ErrInvalidPassword
	}
	userSession, err := service.Authenticate(ctx, plaintext, now)
	if err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	matched, err := service.verifier.Verify(currentPassword, userSession.PasswordHash)
	if err != nil || !matched {
		return LoginResult{}, ErrInvalidCredentials
	}
	newHash, err := service.verifier.Hash(newPassword)
	if err != nil {
		return LoginResult{}, err
	}
	token, err := GenerateSessionToken()
	if err != nil {
		return LoginResult{}, err
	}
	newSession := Session{ID: uuid.New(), UserID: userSession.User.ID, TokenHash: token.Hash, ExpiresAt: now.Add(service.ttl)}
	if err := service.store.ChangePasswordAndRotate(ctx, userSession.User.ID, userSession.SessionID, newHash, newSession, now); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{User: userSession.User, Token: token.Plaintext}, nil
}
