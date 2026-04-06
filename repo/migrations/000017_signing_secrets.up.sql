ALTER TABLE devices ADD COLUMN IF NOT EXISTS signing_secret_enc text;
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS signing_secret_enc text;
