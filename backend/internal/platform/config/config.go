package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const defaultSessionTTL = 168 * time.Hour

type Config struct {
	Environment string
	Addr        string
	Domain      string
	DatabaseURL string
	SessionTTL  time.Duration
	LogLevel    string
}

type LookupFunc func(string) string

func Load(lookup LookupFunc) (Config, error) {
	cfg := Config{
		Environment: strings.TrimSpace(lookup("APP_ENV")),
		Addr:        valueOrDefault(lookup("APP_ADDR"), ":8080"),
		Domain:      strings.TrimSpace(lookup("APP_DOMAIN")),
		DatabaseURL: strings.TrimSpace(lookup("DATABASE_URL")),
		LogLevel:    valueOrDefault(lookup("LOG_LEVEL"), "info"),
		SessionTTL:  defaultSessionTTL,
	}
	// Hosting platforms assign the listener port; it takes priority over APP_ADDR.
	if rawPort := strings.TrimSpace(lookup("PORT")); rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, errors.New("invalid PORT: must be an integer between 1 and 65535")
		}
		cfg.Addr = ":" + strconv.Itoa(port)
	}

	var missing []string
	if cfg.Environment == "" {
		missing = append(missing, "APP_ENV")
	}
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.Environment == "production" && cfg.Domain == "" {
		missing = append(missing, "APP_DOMAIN")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}

	if rawTTL := strings.TrimSpace(lookup("SESSION_TTL")); rawTTL != "" {
		ttl, err := time.ParseDuration(rawTTL)
		if err != nil || ttl <= 0 {
			return Config{}, errors.New("invalid SESSION_TTL: must be a positive duration")
		}
		cfg.SessionTTL = ttl
	}

	return cfg, nil
}

func valueOrDefault(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}
