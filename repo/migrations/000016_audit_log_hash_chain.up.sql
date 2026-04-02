ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS prev_hash text,
    ADD COLUMN IF NOT EXISTS hash text;

UPDATE audit_logs
SET prev_hash = NULL,
    hash = encode(digest(
        coalesce(actor_id::text,'') || '|' ||
        action || '|' ||
        coalesce(resource_type,'') || '|' ||
        coalesce(resource_id::text,'') || '|' ||
        coalesce(detail::text,'') || '|' ||
        to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"') || '|' ||
        ''
    , 'sha256'), 'hex')
WHERE hash IS NULL;

ALTER TABLE audit_logs
    ALTER COLUMN hash SET NOT NULL;
