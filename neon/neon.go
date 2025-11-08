package neon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds the settings required to establish a connection to Neon.
type Config struct {
	DatabaseURL string
	MaxConns    int32
}

// FromEnv builds a Config using process environment variables.
// The primary variable is NEON_DATABASE_URL, which should be provided by Neon.
func FromEnv() (Config, error) {
	url := os.Getenv("NEON_DATABASE_URL")
	if url == "" {
		return Config{}, errors.New("NEON_DATABASE_URL env var is not set")
	}

	var cfg Config
	cfg.DatabaseURL = url

	if maxConns := os.Getenv("NEON_MAX_CONNS"); maxConns != "" {
		val, err := strconv.ParseInt(maxConns, 10, 32)
		if err != nil {
			return Config{}, fmt.Errorf("parse NEON_MAX_CONNS: %w", err)
		}
		cfg.MaxConns = int32(val)
	}

	return cfg, nil
}

// NewPool returns a pgx connection pool configured for Neon.
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if cfg.DatabaseURL == "" {
		return nil, errors.New("database URL is required")
	}

	pgxCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Neon config: %w", err)
	}

	if cfg.MaxConns > 0 {
		pgxCfg.MaxConns = cfg.MaxConns
	}

	// Neon connections can drop when inactive; setting a low max lifetime helps.
	pgxCfg.MaxConnLifetime = 30 * time.Minute
	pgxCfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, pgxCfg)
	if err != nil {
		return nil, fmt.Errorf("connect to Neon: %w", err)
	}

	return pool, nil
}
