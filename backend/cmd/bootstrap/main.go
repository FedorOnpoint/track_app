package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/auth"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/config"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv); err != nil {
		logger.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, err := database.Open(connectCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()
	created, err := auth.NewBootstrapper(auth.NewUserStore(pool), auth.NewPasswordService()).Create(ctx, getenv("BOOTSTRAP_LOGIN"), getenv("BOOTSTRAP_PASSWORD"))
	if err != nil {
		return fmt.Errorf("create bootstrap user: %w", err)
	}
	if created {
		fmt.Fprintln(os.Stdout, "bootstrap user created")
	} else {
		fmt.Fprintln(os.Stdout, "bootstrap user already exists")
	}
	return nil
}
