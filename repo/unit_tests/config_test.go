package unit_tests

import (
	"os"
	"testing"

	"parkops/internal/config"
)

func TestNightlyScheduleDefaults(t *testing.T) {
	// Clear any env overrides
	os.Unsetenv("NIGHTLY_SCHEDULE_HOUR")
	os.Unsetenv("NIGHTLY_SCHEDULE_MINUTE")
	os.Unsetenv("NIGHTLY_SCHEDULE_TIMEZONE")

	os.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	os.Setenv("SESSION_SECRET", "test-secret")
	os.Setenv("ENCRYPTION_KEY", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SESSION_SECRET")
		os.Unsetenv("ENCRYPTION_KEY")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	if cfg.NightlySchedule.Hour != 2 {
		t.Fatalf("expected default hour 2, got %d", cfg.NightlySchedule.Hour)
	}
	if cfg.NightlySchedule.Minute != 0 {
		t.Fatalf("expected default minute 0, got %d", cfg.NightlySchedule.Minute)
	}
	if cfg.NightlySchedule.Timezone.String() != "UTC" {
		t.Fatalf("expected default timezone UTC, got %s", cfg.NightlySchedule.Timezone.String())
	}
}

func TestNightlyScheduleCustomValues(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	os.Setenv("SESSION_SECRET", "test-secret")
	os.Setenv("ENCRYPTION_KEY", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	os.Setenv("NIGHTLY_SCHEDULE_HOUR", "3")
	os.Setenv("NIGHTLY_SCHEDULE_MINUTE", "30")
	os.Setenv("NIGHTLY_SCHEDULE_TIMEZONE", "America/New_York")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SESSION_SECRET")
		os.Unsetenv("ENCRYPTION_KEY")
		os.Unsetenv("NIGHTLY_SCHEDULE_HOUR")
		os.Unsetenv("NIGHTLY_SCHEDULE_MINUTE")
		os.Unsetenv("NIGHTLY_SCHEDULE_TIMEZONE")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	if cfg.NightlySchedule.Hour != 3 {
		t.Fatalf("expected hour 3, got %d", cfg.NightlySchedule.Hour)
	}
	if cfg.NightlySchedule.Minute != 30 {
		t.Fatalf("expected minute 30, got %d", cfg.NightlySchedule.Minute)
	}
	if cfg.NightlySchedule.Timezone.String() != "America/New_York" {
		t.Fatalf("expected America/New_York, got %s", cfg.NightlySchedule.Timezone.String())
	}
}

func TestNightlyScheduleInvalidHour(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	os.Setenv("SESSION_SECRET", "test-secret")
	os.Setenv("ENCRYPTION_KEY", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	os.Setenv("NIGHTLY_SCHEDULE_HOUR", "25")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SESSION_SECRET")
		os.Unsetenv("ENCRYPTION_KEY")
		os.Unsetenv("NIGHTLY_SCHEDULE_HOUR")
	}()

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for hour=25")
	}
}

func TestNightlyScheduleInvalidMinute(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	os.Setenv("SESSION_SECRET", "test-secret")
	os.Setenv("ENCRYPTION_KEY", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	os.Setenv("NIGHTLY_SCHEDULE_MINUTE", "60")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SESSION_SECRET")
		os.Unsetenv("ENCRYPTION_KEY")
		os.Unsetenv("NIGHTLY_SCHEDULE_MINUTE")
	}()

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for minute=60")
	}
}

func TestNightlyScheduleInvalidTimezone(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	os.Setenv("SESSION_SECRET", "test-secret")
	os.Setenv("ENCRYPTION_KEY", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	os.Setenv("NIGHTLY_SCHEDULE_TIMEZONE", "Not/A/Timezone")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SESSION_SECRET")
		os.Unsetenv("ENCRYPTION_KEY")
		os.Unsetenv("NIGHTLY_SCHEDULE_TIMEZONE")
	}()

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}
