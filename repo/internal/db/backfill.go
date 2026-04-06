package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"parkops/internal/platform/security"
)

// BackfillSigningSecrets generates and stores encrypted signing secrets
// for any devices or vehicles that have NULL signing_secret_enc.
// This is idempotent — rows that already have a secret are skipped.
func BackfillSigningSecrets(ctx context.Context, pool *pgxpool.Pool, encryptionKey []byte, logger *slog.Logger) error {
	count, err := backfillTable(ctx, pool, encryptionKey, "devices")
	if err != nil {
		return err
	}
	if count > 0 {
		logger.Info("backfilled device signing secrets", "count", count)
	}

	count, err = backfillTable(ctx, pool, encryptionKey, "vehicles")
	if err != nil {
		return err
	}
	if count > 0 {
		logger.Info("backfilled vehicle signing secrets", "count", count)
	}

	return nil
}

func backfillTable(ctx context.Context, pool *pgxpool.Pool, encryptionKey []byte, table string) (int, error) {
	rows, err := pool.Query(ctx, `SELECT id::text FROM `+table+` WHERE signing_secret_enc IS NULL`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		secretBytes := make([]byte, 32)
		if _, err := rand.Read(secretBytes); err != nil {
			return 0, err
		}
		secretHex := hex.EncodeToString(secretBytes)
		encSecret, err := security.EncryptString(encryptionKey, secretHex)
		if err != nil {
			return 0, err
		}
		_, err = pool.Exec(ctx, `UPDATE `+table+` SET signing_secret_enc = $1 WHERE id = $2::uuid AND signing_secret_enc IS NULL`, encSecret, id)
		if err != nil {
			return 0, err
		}
	}

	return len(ids), nil
}
