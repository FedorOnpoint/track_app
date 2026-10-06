//go:build integration

package auth_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/auth"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestPostgresLoginSessionAndLogout(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:18.6-alpine", postgres.WithDatabase("tracker"), postgres.WithUsername("tracker"), postgres.WithPassword("integration-only"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	testcontainers.CleanupContainer(t, container)
	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	migrator, err := migrate.New("file://"+filepath.ToSlash(migrationsPath(t)), "pgx5"+databaseURL[len("postgres"):])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = migrator.Close() }()
	if err := migrator.Up(); err != nil {
		t.Fatal(err)
	}
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	passwords := auth.NewPasswordService()
	if created, err := auth.NewBootstrapper(auth.NewUserStore(pool), passwords).Create(ctx, "fedor", "integration-password"); err != nil || !created {
		t.Fatalf("bootstrap=(%v,%v)", created, err)
	}
	service := auth.NewService(auth.NewPostgresSessionStore(pool), passwords, time.Hour)
	now := time.Now().UTC()
	login, err := service.Login(ctx, "fedor", "integration-password", now)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := service.Authenticate(ctx, login.Token, now); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if err := service.Logout(ctx, login.Token, now.Add(time.Second)); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := service.Authenticate(ctx, login.Token, now.Add(2*time.Second)); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("authenticate revoked error=%v", err)
	}
}
