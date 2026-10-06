package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/auth"
	"github.com/FedorOnpoint/track_app/backend/internal/bookmarks"
	"github.com/FedorOnpoint/track_app/backend/internal/catalog"
	"github.com/FedorOnpoint/track_app/backend/internal/catalog/catalogdb"
	"github.com/FedorOnpoint/track_app/backend/internal/dashboard"
	"github.com/FedorOnpoint/track_app/backend/internal/notes"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/config"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/httpserver"
	"github.com/FedorOnpoint/track_app/backend/internal/progress"
	"github.com/go-chi/chi/v5"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
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

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	catalogQueries := catalogdb.New(pool)
	progressStore := progress.NewPostgresStore(pool)
	authHandler := auth.NewHandler(auth.NewService(auth.NewPostgresSessionStore(pool), auth.NewPasswordService(), cfg.SessionTTL), cfg.Environment == "production")
	server := httpserver.NewServer(cfg.Addr, httpserver.NewHandler(pool, logger, httpserver.SecurityConfig{Environment: cfg.Environment, Domain: cfg.Domain}, func(router chi.Router) {
		authHandler.Mount(router)
		router.Group(func(protected chi.Router) {
			protected.Use(authHandler.Require)
			catalog.MountRoutes(protected, catalogQueries, progressStore)
			progress.MountRoutes(protected, progressStore)
			bookmarks.MountRoutes(protected, bookmarks.NewPostgresStore(pool))
			notes.MountRoutes(protected, notes.NewPostgresStore(pool))
			dashboard.MountRoutes(protected, dashboard.NewPostgresStore(pool))
		})
	}))
	logger.Info("application started", "addr", cfg.Addr, "environment", cfg.Environment)
	if err := httpserver.Run(ctx, server, listener); err != nil {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}
