package config

import (
	"encoding/hex"
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config holds all environment variables for the application.
type Config struct {
	// Server
	Port     int    `env:"PORT" envDefault:"8080"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// Database
	DatabaseURL string `env:"DATABASE_URL,required"`

	// Redis
	RedisURL string `env:"REDIS_URL,required"`

	// Meta (Instagram)
	MetaAppSecret       string `env:"META_APP_SECRET,required"`
	MetaGraphAPIVersion string `env:"META_GRAPH_API_VERSION" envDefault:"v25.0"`

	// Backblaze B2 (S3-compatible)
	B2Endpoint       string `env:"B2_ENDPOINT,required"`
	B2Region         string `env:"B2_REGION,required"`
	B2Bucket         string `env:"B2_BUCKET,required"`
	B2KeyID          string `env:"B2_KEY_ID,required"`
	B2ApplicationKey string `env:"B2_APPLICATION_KEY,required"`
	B2PublicURL      string `env:"B2_PUBLIC_URL,required"`
	B2Prefix         string `env:"B2_PREFIX" envDefault:"instasae"`

	// Security
	EncryptionKey      string `env:"ENCRYPTION_KEY,required"`
	AdminAPIKey        string `env:"ADMIN_API_KEY,required"`
	WebhookVerifyToken string `env:"WEBHOOK_VERIFY_TOKEN,required"`
}

// Load parses environment variables into Config and validates them.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing env vars: %w", err)
	}

	if err := validateEncryptionKey(cfg.EncryptionKey); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validateEncryptionKey(key string) error {
	if len(key) != 64 {
		return fmt.Errorf("ENCRYPTION_KEY must be exactly 64 hex characters (got %d)", len(key))
	}
	if _, err := hex.DecodeString(key); err != nil {
		return fmt.Errorf("ENCRYPTION_KEY must be valid hex: %w", err)
	}
	return nil
}
