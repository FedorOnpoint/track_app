package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type memorySessions struct {
	user      User
	created   Session
	revokedID uuid.UUID
}

func (store *memorySessions) FindUserByLogin(_ context.Context, login string) (User, error) {
	if store.user.Login != login {
		return User{}, pgx.ErrNoRows
	}
	return store.user, nil
}
func (store *memorySessions) CreateSession(_ context.Context, session Session) error {
	store.created = session
	return nil
}
func (store *memorySessions) FindUserByTokenHash(_ context.Context, hash []byte, _ time.Time) (UserSession, error) {
	if len(hash) != 32 || store.created.ID == uuid.Nil {
		return UserSession{}, pgx.ErrNoRows
	}
	return UserSession{User: store.user, SessionID: store.created.ID}, nil
}
func (store *memorySessions) RevokeSession(_ context.Context, id uuid.UUID, _ time.Time) error {
	store.revokedID = id
	return nil
}

type recordingVerifier struct {
	calls int
	match bool
}

func (verifier *recordingVerifier) Verify(string, string) (bool, error) {
	verifier.calls++
	return verifier.match, nil
}

func TestLoginCreatesHashedSessionToken(t *testing.T) {
	store := &memorySessions{user: User{ID: uuid.New(), Login: "fedor", PasswordHash: "encoded"}}
	verifier := &recordingVerifier{match: true}
	service := NewService(store, verifier, time.Hour)
	result, err := service.Login(context.Background(), "fedor", "secret-password", time.Now())
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if result.Token == "" {
		t.Fatal("Login returned empty token")
	}
	if len(store.created.TokenHash) != 32 {
		t.Fatalf("stored token hash length = %d", len(store.created.TokenHash))
	}
	if store.created.UserID != store.user.ID {
		t.Fatalf("session user = %s", store.created.UserID)
	}
}

func TestLoginUsesSameVerificationPathForUnknownUser(t *testing.T) {
	verifier := &recordingVerifier{}
	service := NewService(&memorySessions{}, verifier, time.Hour)
	_, err := service.Login(context.Background(), "unknown", "secret-password", time.Now())
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login error = %v", err)
	}
	if verifier.calls != 1 {
		t.Fatalf("Verify calls = %d, want 1", verifier.calls)
	}
}
