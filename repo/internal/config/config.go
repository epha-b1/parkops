package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type NightlySchedule struct {
	Hour     int
	Minute   int
	Timezone *time.Location
}

type Config struct {
	AppAddr         string
	AppEnv          string
	DatabaseURL     string
	EncryptionKey   []byte
	SessionSecret   string
	NightlySchedule NightlySchedule
	ExportStorageDir string
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

	// Nightly schedule config with safe defaults
	ns, err := loadNightlySchedule()
	if err != nil {
		return Config{}, err
	}
	cfg.NightlySchedule = ns
	cfg.ExportStorageDir = getEnv("EXPORT_STORAGE_DIR", "data/exports")

	return cfg, nil
}

func loadNightlySchedule() (NightlySchedule, error) {
	hourStr := getEnv("NIGHTLY_SCHEDULE_HOUR", "2")
	minuteStr := getEnv("NIGHTLY_SCHEDULE_MINUTE", "0")
	tzStr := getEnv("NIGHTLY_SCHEDULE_TIMEZONE", "UTC")

	hour, err := strconv.Atoi(hourStr)
	if err != nil || hour < 0 || hour > 23 {
		return NightlySchedule{}, fmt.Errorf("NIGHTLY_SCHEDULE_HOUR must be 0-23, got %q", hourStr)
	}

	minute, err := strconv.Atoi(minuteStr)
	if err != nil || minute < 0 || minute > 59 {
		return NightlySchedule{}, fmt.Errorf("NIGHTLY_SCHEDULE_MINUTE must be 0-59, got %q", minuteStr)
	}

	loc, err := time.LoadLocation(tzStr)
	if err != nil {
		return NightlySchedule{}, fmt.Errorf("NIGHTLY_SCHEDULE_TIMEZONE invalid: %w", err)
	}

	return NightlySchedule{Hour: hour, Minute: minute, Timezone: loc}, nil
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}
