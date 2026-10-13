package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/platform/config"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
	"github.com/FedorOnpoint/track_app/backend/internal/seedcontent"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	ctx, timeout := context.WithTimeout(ctx, 2*time.Minute)
	defer timeout()
	if err := run(ctx, os.Getenv, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("usage: seed category.json [category.json ...]; apply migrations first")
	}
	documents := make([]seedcontent.Document, 0, len(paths))
	topics := 0
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		doc, err := seedcontent.Decode(file)
		file.Close()
		if err == nil {
			err = seedcontent.Validate(doc)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		documents = append(documents, doc)
		topics += len(doc.Topics)
	}
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, err := database.Open(connectCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("cannot connect to database; verify DATABASE_URL and database availability")
	}
	defer pool.Close()
	if err := seedcontent.Import(ctx, pool, documents); err != nil {
		return fmt.Errorf("seed import rolled back: %w", err)
	}
	fmt.Printf("seed imported: %d categories, %d topics\n", len(documents), topics)
	return nil
}
