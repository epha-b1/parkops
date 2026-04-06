# ParkOps

Start the platform:

`docker compose up --build`

Run the platform without Docker:

`DATABASE_URL='postgres://parkops:parkops@127.0.0.1:5432/parkops?sslmode=disable' SESSION_SECRET='dev-session-secret' ENCRYPTION_KEY='00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff' go run ./cmd/server`

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | Yes | — | PostgreSQL connection string |
| `SESSION_SECRET` | Yes | — | Secret for session management |
| `ENCRYPTION_KEY` | Yes | — | 64-char hex string (32 bytes) for AES-256-GCM encryption of signing secrets |
| `APP_ADDR` | No | `:8080` | HTTP listen address |
| `APP_ENV` | No | `development` | Environment (`development` or `production`) |
| `NIGHTLY_SCHEDULE_HOUR` | No | `2` | Hour (0-23) for nightly segment scheduler |
| `NIGHTLY_SCHEDULE_MINUTE` | No | `0` | Minute (0-59) for nightly segment scheduler |
| `NIGHTLY_SCHEDULE_TIMEZONE` | No | `UTC` | IANA timezone for nightly scheduler (e.g. `America/New_York`) |
| `EXPORT_STORAGE_DIR` | No | `data/exports` | Directory for export file storage (created automatically) |

## Dependencies

- `github.com/xuri/excelize/v2` — real XLSX binary export generation
- `github.com/go-pdf/fpdf` — real PDF binary export generation

## Migration Notes

- Migration `000017_signing_secrets` adds `signing_secret_enc` columns to the `devices` and `vehicles` tables. These store AES-256-GCM encrypted HMAC signing secrets.
- On startup, the server runs an idempotent backfill that generates encrypted signing secrets for any existing devices/vehicles with `NULL` values. No manual intervention is needed.
- New devices and vehicles automatically receive generated signing secrets on creation.

Run all tests:

`run_tests.sh`

Run tests without Docker (unit + API):

```bash
TEST_DATABASE_URL='postgres://parkops:parkops@127.0.0.1:5432/parkops?sslmode=disable' go test -mod=mod ./unit_tests/... ./API_tests/... -v -count=1
```

Access the login page:

`http://localhost:8080/login`

## Development-Only Seed Credentials

All seeded accounts require a password change on first login (`force_password_change=true`). These credentials exist only for local development and testing. Do not use in production.

**Login URL**: http://localhost:8080/login

**Admin Account**:
- **Username**: `admin`
- **Password**: `AdminPass1234` (must change on first login)

**Demo Users** (seeded by migration 000015, dev-only):
- Dispatch Operator: `operator1` / `AdminPass1234`
- Fleet Manager: `fleet1` / `AdminPass1234`
- Auditor: `auditor1` / `AdminPass1234`

**API Testing with curl**:
```bash
# Login and save session cookie
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "AdminPass1234"}' \
  -c cookies.txt

# Change password (required on first login)
curl -X PATCH http://localhost:8080/api/me/password \
  -H "Content-Type: application/json" \
  -d '{"current_password": "AdminPass1234", "new_password": "NewSecurePass1234"}' \
  -b cookies.txt

# Test authenticated endpoint
curl -X GET http://localhost:8080/api/me \
  -b cookies.txt
```
