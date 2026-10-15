package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryRegistration struct {
	attemptErr  error
	registerErr error
	attempts    int
	user        User
	session     Session
}

func (s *memoryRegistration) TakeInvitationAttempt(context.Context, []byte, time.Time) error {
	s.attempts++
	return s.attemptErr
}
func (s *memoryRegistration) RegisterWithInvitation(_ context.Context, user User, session Session, _ []byte, _ time.Time) error {
	if s.registerErr != nil {
		return s.registerErr
	}
	s.user = user
	s.session = session
	return nil
}

func TestRegistrationCreatesUserAndHashedSession(t *testing.T) {
	invite, _ := GenerateSessionToken()
	store := &memoryRegistration{}
	service := NewRegistrationService(store, &recordingVerifier{}, time.Hour)
	result, err := service.Register(context.Background(), "new-user", "long-password-123", invite.Plaintext, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if result.User.Login != "new-user" || store.user.PasswordHash != "new-encoded-hash" {
		t.Fatal("user not created with hashed password")
	}
	hash, err := HashSessionToken(result.Token)
	if err != nil || string(hash) != string(store.session.TokenHash) || store.session.UserID != store.user.ID {
		t.Fatal("invalid session")
	}
}

func TestRegistrationRejectsInvalidInputsBeforeStore(t *testing.T) {
	invite, _ := GenerateSessionToken()
	for _, tc := range []struct {
		login, password, code string
		want                  error
	}{
		{"UPPER", "long-password-123", invite.Plaintext, ErrInvalidLogin},
		{"new-user", "short", invite.Plaintext, ErrInvalidPassword},
		{"new-user", "long-password-123", "invalid-code", ErrInvalidInvitation},
	} {
		store := &memoryRegistration{}
		_, err := NewRegistrationService(store, &recordingVerifier{}, time.Hour).Register(context.Background(), tc.login, tc.password, tc.code, time.Now())
		if !errors.Is(err, tc.want) || store.attempts != 0 {
			t.Fatalf("error=%v attempts=%d", err, store.attempts)
		}
	}
}

func TestRegistrationPropagatesInvitationAndLoginFailures(t *testing.T) {
	invite, _ := GenerateSessionToken()
	for _, want := range []error{ErrInvalidInvitation, ErrRegistrationRateLimited, ErrLoginUnavailable} {
		store := &memoryRegistration{}
		if want == ErrLoginUnavailable {
			store.registerErr = want
		} else {
			store.attemptErr = want
		}
		_, err := NewRegistrationService(store, &recordingVerifier{}, time.Hour).Register(context.Background(), "new-user", "long-password-123", invite.Plaintext, time.Now())
		if !errors.Is(err, want) || store.user.Login != "" {
			t.Fatalf("error=%v user created=%v", err, store.user.Login != "")
		}
	}
}
