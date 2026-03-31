package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

type Config struct {
	AppAddr       string
	AppEnv        string
	DatabaseURL   string
	EncryptionKey []byte
	SessionSecret string
}

func Load() (Config, error) {
	cfg := Config{
		AppAddr:       getEnv("APP_ADDR", ":8080"),
		AppEnv:        getEnv("APP_ENV", "development"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.SessionSecret == "" {
		return Config{}, errors.New("SESSION_SECRET is required")
	}

	keyHex := os.Getenv("ENCRYPTION_KEY")
	if keyHex == "" {
		return Config{}, errors.New("ENCRYPTION_KEY is required")
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return Config{}, fmt.Errorf("ENCRYPTION_KEY must be hex: %w", err)
	}
	if len(key) != 32 {
		return Config{}, fmt.Errorf("ENCRYPTION_KEY must decode to 32 bytes, got %d", len(key))
	}
	cfg.EncryptionKey = key

	return cfg, nil
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}
