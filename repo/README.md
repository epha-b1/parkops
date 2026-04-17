Project Type: fullstack

# ParkOps

Offline-first parking operations platform. Fullstack (Go backend + server-rendered Templ UI) served from a single Docker Compose stack.

## Startup

```bash
docker-compose up
```

(`docker compose up --build` also works on newer Docker CLIs.)

## Access

- **App URL**: http://localhost:8080
- **Login page**: http://localhost:8080/login
- **Health check**: http://localhost:8080/api/health

## Architecture Overview

- **Backend**: Go + Gin HTTP router, PostgreSQL 16
- **Frontend**: server-rendered [Templ](https://templ.guide) pages under `internal/web/*.go` (no SPA, no separate build)
- **Auth**: cookie session with Argon2id password hashing, HMAC-signed session cookies
- **Exports**: binary XLSX (`excelize/v2`) and PDF (`go-pdf/fpdf`), file-based storage
- **Schedulers**: reconciliation, notifications, campaign reminders, nightly segment evaluation
- **Migrations**: `golang-migrate`, files under `migrations/`

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | Yes | — | PostgreSQL connection string |
| `SESSION_SECRET` | Yes | — | HMAC key used to sign session cookies |
| `ENCRYPTION_KEY` | Yes | — | 64-char hex (32 bytes) for AES-256-GCM encryption of signing secrets |
| `APP_ADDR` | No | `:8080` | HTTP listen address |
| `APP_ENV` | No | `development` | Environment (`development` or `production` enables `Secure` cookies) |
| `NIGHTLY_SCHEDULE_HOUR` | No | `2` | Hour (0-23) for nightly segment scheduler |
| `NIGHTLY_SCHEDULE_MINUTE` | No | `0` | Minute (0-59) for nightly segment scheduler |
| `NIGHTLY_SCHEDULE_TIMEZONE` | No | `UTC` | IANA timezone for nightly scheduler (e.g. `America/New_York`) |
| `EXPORT_STORAGE_DIR` | No | `data/exports` | Directory for export file storage (created automatically) |

Default values are wired into `docker-compose.yml` so `docker-compose up` is sufficient for local runs.

## Migration Notes

- `000017_signing_secrets` adds encrypted `signing_secret_enc` columns to `devices` and `vehicles`.
- On startup, the server idempotently backfills secrets for any legacy rows with `NULL` values.
- New devices/vehicles automatically receive generated signing secrets on creation.

## Tests

All tests run inside the Docker stack — no local Go toolchain required.

```bash
./run_tests.sh
```

This script starts the stack (if needed), waits for `/api/health`, then runs the full unit + API suite via `docker compose exec`. Test output is printed to stdout.

## Development-Only Seed Credentials

All seeded accounts are marked `force_password_change=true`; first login requires a password rotation. These accounts exist purely for local development and demos. **Do not use in production.**

| Role | Username | Password | Typical Workflow |
|---|---|---|---|
| Facility Admin | `admin` | `AdminPass1234` | full platform administration: facility/lot/zone CRUD, user management, audit log, segment/campaign creation, reconciliation |
| Dispatch Operator | `operator1` | `AdminPass1234` | reservation lifecycle (hold/confirm/cancel), device event triage, exception acknowledgement, exports |
| Fleet Manager | `fleet1` | `AdminPass1234` | org-scoped vehicle/driver/member management, tracking |
| Auditor | `auditor1` | `AdminPass1234` | read-only access to audit logs and operational views |

## API Verification (curl)

```bash
# 1. Login and save session cookie
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "AdminPass1234"}' \
  -c cookies.txt

# 2. Change password (required on first login)
curl -X PATCH http://localhost:8080/api/me/password \
  -H "Content-Type: application/json" \
  -d '{"current_password": "AdminPass1234", "new_password": "NewSecurePass1234"}' \
  -b cookies.txt

# 3. Authenticated endpoint
curl -X GET http://localhost:8080/api/me -b cookies.txt

# 4. Admin endpoint
curl -X GET http://localhost:8080/api/admin/users -b cookies.txt
```

## UI Validation Walkthrough

After `docker-compose up`, visit `http://localhost:8080/login` and log in as `admin` / `AdminPass1234`. You will be prompted to change the password. After changing, explore:

1. `/dashboard` — KPI overview
2. `/reservations` — create a hold, confirm, cancel
3. `/capacity` — live zone capacity dashboard
4. `/notifications` — view delivered notifications
5. `/notification-prefs` — topic subscriptions + DND controls
6. `/tasks` — campaign task management
7. `/analytics` — occupancy/booking/exception charts + CSV/XLSX/PDF exports
8. `/audit` (admin) — tamper-evident audit log with hash chain
