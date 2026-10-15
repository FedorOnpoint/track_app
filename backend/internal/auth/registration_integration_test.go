//go:build integration

package auth_test

import (
	"context"
	"errors"
	"github.com/FedorOnpoint/track_app/backend/internal/auth"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
	"github.com/golang-migrate/migrate/v4"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestPostgresInvitedRegistration(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:18.6-alpine", postgres.WithDatabase("tracker"), postgres.WithUsername("tracker"), postgres.WithPassword("integration-only"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	testcontainers.CleanupContainer(t, container)
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.New("file://"+filepath.ToSlash(migrationsPath(t)), "pgx5"+dsn[len("postgres"):])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := auth.NewPostgresRegistrationStore(pool)
	passwords := auth.NewPasswordService()
	service := auth.NewRegistrationService(store, passwords, time.Hour)
	sessions := auth.NewService(auth.NewPostgresSessionStore(pool), passwords, time.Hour)
	newInvite := func() auth.SessionToken {
		t.Helper()
		code, err := auth.GenerateSessionToken()
		if err != nil {
			t.Fatal(err)
		}
		if err := store.CreateInvitation(ctx, code.Hash, time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		return code
	}
	code := newInvite()
	registered, err := service.Register(ctx, "alice", "integration-password", code.Plaintext, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.Authenticate(ctx, registered.Token, time.Now()); err != nil {
		t.Fatal(err)
	}
	matched, err := passwords.Verify("integration-password", registered.User.PasswordHash)
	if err != nil || !matched {
		t.Fatal("invalid stored password hash")
	}
	if _, err := service.Register(ctx, "bob", "integration-password", code.Plaintext, time.Now()); !errors.Is(err, auth.ErrInvalidInvitation) {
		t.Fatalf("reused invite: %v", err)
	}
	var personalRows int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM notes WHERE user_id=$1)+(SELECT count(*) FROM bookmarks WHERE user_id=$1)+(SELECT count(*) FROM topic_progress WHERE user_id=$1)`, registered.User.ID).Scan(&personalRows); err != nil {
		t.Fatal(err)
	}
	if personalRows != 0 {
		t.Fatal("new account has another user's data")
	}

	code = newInvite()
	if _, err := service.Register(ctx, "alice", "other-password-123", code.Plaintext, time.Now()); !errors.Is(err, auth.ErrLoginUnavailable) {
		t.Fatalf("duplicate login: %v", err)
	}
	if _, err := service.Register(ctx, "bob", "integration-password", code.Plaintext, time.Now()); err != nil {
		t.Fatalf("duplicate login consumed invite: %v", err)
	}
	if _, err := sessions.Login(ctx, "alice", "integration-password", time.Now()); err != nil {
		t.Fatal("existing password was modified")
	}

	code = newInvite()
	if _, err := service.Register(ctx, "expired", "integration-password", code.Plaintext, time.Now().Add(2*time.Hour)); !errors.Is(err, auth.ErrInvalidInvitation) {
		t.Fatalf("expired invitation: %v", err)
	}
	code = newInvite()
	for i := 0; i < 10; i++ {
		if err := store.TakeInvitationAttempt(ctx, code.Hash, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.TakeInvitationAttempt(ctx, code.Hash, time.Now()); !errors.Is(err, auth.ErrRegistrationRateLimited) {
		t.Fatalf("limit: %v", err)
	}
	if err := store.TakeInvitationAttempt(ctx, code.Hash, time.Now().Add(16*time.Minute)); err != nil {
		t.Fatalf("quota reset: %v", err)
	}

	code = newInvite()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, login := range []string{"racer-one", "racer-two"} {
		wg.Add(1)
		go func(login string) {
			defer wg.Done()
			_, err := service.Register(ctx, login, "integration-password", code.Plaintext, time.Now())
			results <- err
		}(login)
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, auth.ErrInvalidInvitation) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent successes=%d", successes)
	}

	// A session insert failure must roll back both the account and invite consumption.
	code = newInvite()
	user := auth.User{ID: uuid.New(), Login: "rolled-back", PasswordHash: registered.User.PasswordHash}
	badSession := auth.Session{ID: uuid.New(), UserID: user.ID, TokenHash: []byte{1}, ExpiresAt: time.Now().Add(time.Hour)}
	if err := store.RegisterWithInvitation(ctx, user, badSession, code.Hash, time.Now()); err == nil {
		t.Fatal("invalid session succeeded")
	}
	if _, err := service.Register(ctx, "rolled-back", "integration-password", code.Plaintext, time.Now()); err != nil {
		t.Fatalf("failed transaction consumed login or invite: %v", err)
	}
}
