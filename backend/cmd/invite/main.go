package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/FedorOnpoint/track_app/backend/internal/auth"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
	"os"
	"time"
)

func run() error {
	ttl := flag.Duration("ttl", 7*24*time.Hour, "invitation lifetime (maximum 90 days)")
	flag.Parse()
	if *ttl <= 0 || *ttl > 90*24*time.Hour || flag.NArg() != 0 {
		return fmt.Errorf("ttl must be between 0 and 2160h; no positional arguments are accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("cannot connect to database; verify DATABASE_URL")
	}
	defer pool.Close()
	code, err := auth.GenerateSessionToken()
	if err != nil {
		return fmt.Errorf("cannot generate invitation")
	}
	if err := auth.NewPostgresRegistrationStore(pool).CreateInvitation(ctx, code.Hash, time.Now().Add(*ttl)); err != nil {
		return fmt.Errorf("cannot create invitation; apply migrations first")
	}
	// Only the administrator command prints the plaintext code; the DB stores its hash.
	fmt.Println(code.Plaintext)
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
