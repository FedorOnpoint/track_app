package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ParsePoolConfig(databaseURL string) (*pgxpool.Config, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	config.MaxConns = 10
	config.MinIdleConns = 1
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 15 * time.Minute
	config.HealthCheckPeriod = time.Minute
	config.PingTimeout = 2 * time.Second
	return config, nil
}

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := ParsePoolConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
