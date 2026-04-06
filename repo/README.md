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

## Migration Notes

Migration `000017_signing_secrets` adds `signing_secret_enc` columns to the `devices` and `vehicles` tables. These store AES-256-GCM encrypted HMAC signing secrets. Existing devices/vehicles will have `NULL` signing secrets until re-registered or updated. New devices and vehicles automatically receive generated signing secrets on creation.

Run all tests:

`run_tests.sh`

Run tests without Docker (unit + API):

```bash
TEST_DATABASE_URL='postgres://parkops:parkops@127.0.0.1:5432/parkops?sslmode=disable' go test -mod=mod ./unit_tests/... ./API_tests/... -v -count=1
```

Access the login page:

`http://localhost:8080/login`

## Default Admin Credentials

**Login URL**: http://localhost:8080/login

**Admin Account**:
- **Username**: `admin`
- **Password**: `AdminPass1234`

**API Testing with curl**:
```bash
# Login and save session cookie
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "AdminPass1234"}' \
  -c cookies.txt

# Test authenticated endpoint
curl -X GET http://localhost:8080/api/me \
  -b cookies.txt

# Test admin endpoint
curl -X GET http://localhost:8080/api/admin/users \
  -b cookies.txt
```

**Additional Test Users** (create via API after admin login):
- Fleet Manager: `fleet1` / `FleetPass1234`
- Dispatch Operator: `dispatch1` / `DispatchPass1234`
- Auditor: `auditor1` / `AuditorPass1234`
