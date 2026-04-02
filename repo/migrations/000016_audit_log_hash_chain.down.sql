ALTER TABLE audit_logs
    DROP COLUMN IF EXISTS hash,
    DROP COLUMN IF EXISTS prev_hash;
