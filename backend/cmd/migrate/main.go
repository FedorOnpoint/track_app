package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func migrationURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return "", errors.New("DATABASE_URL must be a PostgreSQL URL")
	}
	parsed.Scheme = "pgx5"
	return parsed.String(), nil
}
func run() error {
	dsn, err := migrationURL(os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	path := os.Getenv("MIGRATIONS_DIR")
	if path == "" {
		path = "/app/migrations"
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return errors.New("invalid migration directory")
	}
	m, err := migrate.New("file://"+filepath.ToSlash(path), dsn)
	if err != nil {
		return errors.New("cannot initialize migrations; verify database availability and migration directory")
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return errors.New("migration failed; inspect database migration state before retrying")
	}
	fmt.Println("migrations applied")
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
