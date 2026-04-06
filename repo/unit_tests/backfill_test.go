package unit_tests

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"parkops/internal/db"
	"parkops/internal/platform/security"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestBackfillSigningSecretsIdempotent(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://parkops:parkops@127.0.0.1:5432/parkops?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("db unavailable: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("db unreachable: %v", err)
	}

	encKey := []byte("0123456789abcdef0123456789abcdef")

	// Insert a vehicle with NULL signing_secret_enc to simulate legacy row
	_, err = pool.Exec(ctx, `
		INSERT INTO vehicles(organization_id, plate_number, make, model, signing_secret_enc)
		VALUES ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid, 'BACKFILL-TEST', 'Test', 'Car', NULL)
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		t.Fatalf("insert legacy vehicle: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM vehicles WHERE plate_number = 'BACKFILL-TEST'`)
	})

	// Verify it's NULL
	var secretBefore *string
	err = pool.QueryRow(ctx, `SELECT signing_secret_enc FROM vehicles WHERE plate_number='BACKFILL-TEST'`).Scan(&secretBefore)
	if err != nil {
		t.Fatalf("query before: %v", err)
	}
	if secretBefore != nil {
		t.Fatal("expected NULL signing_secret_enc before backfill")
	}

	// Run backfill
	logger := testLogger()
	err = db.BackfillSigningSecrets(ctx, pool, encKey, logger)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}

	// Verify it's populated and decryptable
	var secretAfter *string
	err = pool.QueryRow(ctx, `SELECT signing_secret_enc FROM vehicles WHERE plate_number='BACKFILL-TEST'`).Scan(&secretAfter)
	if err != nil {
		t.Fatalf("query after: %v", err)
	}
	if secretAfter == nil {
		t.Fatal("expected non-NULL signing_secret_enc after backfill")
	}
	decrypted, err := security.DecryptString(encKey, *secretAfter)
	if err != nil {
		t.Fatalf("decrypt backfilled secret: %v", err)
	}
	if len(decrypted) != 64 { // 32 bytes hex-encoded
		t.Fatalf("expected 64-char hex secret, got %d chars", len(decrypted))
	}

	// Run backfill again — should be idempotent (same value, not overwritten)
	err = db.BackfillSigningSecrets(ctx, pool, encKey, logger)
	if err != nil {
		t.Fatalf("backfill idempotent: %v", err)
	}
	var secretAfter2 *string
	err = pool.QueryRow(ctx, `SELECT signing_secret_enc FROM vehicles WHERE plate_number='BACKFILL-TEST'`).Scan(&secretAfter2)
	if err != nil {
		t.Fatalf("query after2: %v", err)
	}
	if secretAfter2 == nil || *secretAfter2 != *secretAfter {
		t.Fatal("backfill should be idempotent — secret changed on re-run")
	}
}
