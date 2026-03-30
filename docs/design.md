# ParkOps — Design Document

## 1. Overview

ParkOps is an offline-first, fullstack parking operations platform. It runs entirely on a local network with no external connectivity. The backend is a single Go (Gin) process that serves both a Templ-rendered web UI and a REST API. PostgreSQL is the sole system of record.

---

## 2. Architecture

```
Browser
  │
  ▼
Gin HTTP Server (port 8080)
  ├── Templ page handlers   → full-page SSR responses
  ├── REST API handlers     → JSON responses for HTMX/JS polling
  ├── RBAC middleware       → role check on every route
  ├── Session middleware    → inactivity timeout, force_password_change
  └── Audit middleware      → append-only log for sensitive operations
        │
        ▼
   Service layer           → business logic, no HTTP concerns
        │
        ▼
   Repository layer        → SQL queries via pgx
        │
        ▼
   PostgreSQL (port 5432)
```

No microservices. No message broker. One process, one database. Background jobs run as goroutines inside the same process, managed by the internal scheduler.

---

## 3. Technology Stack

| Layer | Choice | Reason |
|---|---|---|
| HTTP framework | Gin | fast, minimal, good middleware support |
| UI rendering | Templ | type-safe SSR templates in Go |
| Live updates | HTTP polling (10–15s) | simple, no WebSocket complexity for MVP |
| DB driver | pgx v5 | native PostgreSQL driver, good performance |
| Query layer | sqlc | type-safe generated queries from SQL |
| Migrations | golang-migrate | sequential numbered SQL files |
| Logging | slog (stdlib) | structured, no extra dependency |
| Encryption | AES-256-GCM | authenticated encryption for sensitive fields |
| Password hashing | argon2id | memory-hard, resistant to GPU attacks |
| CSS | Tailwind CSS (CDN) | utility-first, no build step needed |
| Container | Docker + docker-compose | one-click startup |

---

## 4. Module Responsibilities

| Module | Responsibility |
|---|---|
| `auth` | Login, logout, password hashing, lockout counter, session create/expire |
| `rbac` | Role check middleware, permission model |
| `users` | User CRUD, admin management |
| `facilities` | Facility CRUD |
| `zones` | Zone CRUD, stall count, hold timeout config |
| `rates` | Rate plan CRUD, linked to zone |
| `members` | Member CRUD, arrears balance, org scoping |
| `vehicles` | Vehicle CRUD, org scoped |
| `drivers` | Driver CRUD, org scoped |
| `reservations` | Hold creation, confirmation, cancellation, calendar queries |
| `capacity` | Stall count tracking, dashboard queries, reconciliation job |
| `exceptions` | Device exception lifecycle, acknowledgement |
| `devices` | Device registration, event ingestion, idempotency, replay |
| `tracking` | Location ingestion, drift smoothing, stop detection |
| `notifications` | Topic subscriptions, job queue, DND, frequency cap, dispatcher |
| `campaigns` | Campaign and task CRUD |
| `tasks` | Task completion, reminder job generation |
| `tags` | Tag CRUD, member tag assignment, version export/import |
| `segments` | Segment definition, preview, on-demand and nightly evaluation |
| `analytics` | Pivot queries, trend data |
| `exports` | CSV/Excel/PDF generation, role + segment access checks |
| `audit` | Append-only audit log writes |
| `jobs` | Background job scheduler, worker loop |
| `db` | Connection pool, transaction helpers, migration runner |
| `web` | Templ templates, static assets, page handlers |
| `platform` | Logger, clock, encryption, pagination, validator |

---

## 5. Data Model

### Auth and Users

```
users
  id uuid PK
  username text UNIQUE NOT NULL
  password_hash text NOT NULL          -- argon2id, never raw
  display_name text
  status text                          -- active | inactive | locked
  failed_login_count int DEFAULT 0
  locked_until timestamptz
  force_password_change bool DEFAULT false
  created_at timestamptz
  updated_at timestamptz

roles
  id uuid PK
  name text UNIQUE                     -- facility_admin | dispatch_operator | fleet_manager | auditor

user_roles
  user_id uuid FK users
  role_id uuid FK roles
  PRIMARY KEY (user_id, role_id)

sessions
  id uuid PK
  user_id uuid FK users
  created_at timestamptz
  last_active_at timestamptz
  expires_at timestamptz
```

### Master Data

```
facilities
  id uuid PK
  name text NOT NULL
  address text
  created_at timestamptz

lots
  id uuid PK
  facility_id uuid FK facilities
  name text NOT NULL

zones
  id uuid PK
  lot_id uuid FK lots
  name text NOT NULL
  total_stalls int NOT NULL
  hold_timeout_minutes int DEFAULT 15

rate_plans
  id uuid PK
  zone_id uuid FK zones
  name text NOT NULL
  rate_cents int NOT NULL
  period text                          -- hourly | daily | monthly

members
  id uuid PK
  organization_id uuid NOT NULL
  display_name text NOT NULL
  contact_notes_enc text               -- AES-256-GCM encrypted
  arrears_balance_cents int DEFAULT 0
  created_at timestamptz

vehicles
  id uuid PK
  organization_id uuid NOT NULL
  plate_number text NOT NULL
  make text
  model text

drivers
  id uuid PK
  organization_id uuid NOT NULL
  member_id uuid FK members
  licence_number text

message_rules
  id uuid PK
  trigger_event text NOT NULL          -- booking.confirmed | booking.changed | expiry.approaching | arrears.reminder
  topic_id uuid FK notification_topics
  template text NOT NULL
  active bool DEFAULT true
```

### Reservations and Capacity

```
reservations
  id uuid PK
  zone_id uuid FK zones
  member_id uuid FK members
  vehicle_id uuid FK vehicles
  status text                          -- hold | confirmed | cancelled | expired
  time_window_start timestamptz NOT NULL
  time_window_end timestamptz NOT NULL
  stall_count int NOT NULL
  rate_plan_id uuid FK rate_plans
  created_at timestamptz
  confirmed_at timestamptz
  cancelled_at timestamptz

capacity_holds
  id uuid PK
  reservation_id uuid FK reservations
  zone_id uuid FK zones
  stall_count int NOT NULL
  expires_at timestamptz NOT NULL
  released_at timestamptz

capacity_buckets
  id uuid PK
  zone_id uuid FK zones
  time_window_start timestamptz NOT NULL
  time_window_end timestamptz NOT NULL
  available_stalls int NOT NULL
  UNIQUE (zone_id, time_window_start, time_window_end)

capacity_snapshots
  id uuid PK
  zone_id uuid FK zones
  snapshot_at timestamptz NOT NULL
  authoritative_stalls int NOT NULL

booking_events
  id uuid PK
  reservation_id uuid FK reservations
  event_type text NOT NULL             -- hold_created | confirmed | cancelled | expired | hold_released
  occurred_at timestamptz NOT NULL
  actor_id uuid FK users

reconciliation_runs
  id uuid PK
  started_at timestamptz NOT NULL
  completed_at timestamptz
  zones_checked int
  discrepancies_found int

compensating_events
  id uuid PK
  reconciliation_run_id uuid FK reconciliation_runs
  zone_id uuid FK zones
  delta int NOT NULL                   -- positive = release, negative = hold
  reason text
  applied_at timestamptz
```

### Devices and Tracking

```
devices
  id uuid PK
  device_key text UNIQUE NOT NULL      -- pre-shared HMAC key for signed timestamps
  device_type text NOT NULL            -- camera | gate | geomagnetic
  zone_id uuid FK zones
  status text                          -- online | offline
  registered_at timestamptz

device_events
  id uuid PK
  device_id uuid FK devices
  event_key text UNIQUE NOT NULL       -- idempotency key
  sequence_number bigint NOT NULL
  event_type text NOT NULL
  payload jsonb
  received_at timestamptz NOT NULL     -- server time
  device_time timestamptz              -- signed device time (nullable)
  device_time_trusted bool DEFAULT false
  late bool DEFAULT false
  processed bool DEFAULT false

exceptions
  id uuid PK
  device_id uuid FK devices
  exception_type text NOT NULL         -- gate_stuck | sensor_offline | camera_error
  status text DEFAULT 'open'           -- open | acknowledged
  acknowledged_by uuid FK users
  acknowledged_at timestamptz
  note text
  created_at timestamptz

vehicle_positions
  id uuid PK
  vehicle_id uuid FK vehicles
  latitude decimal NOT NULL
  longitude decimal NOT NULL
  received_at timestamptz NOT NULL
  device_time timestamptz
  device_time_trusted bool DEFAULT false
  suspect bool DEFAULT false           -- drift smoothing flag

stop_events
  id uuid PK
  vehicle_id uuid FK vehicles
  started_at timestamptz NOT NULL
  detected_at timestamptz NOT NULL
  latitude decimal
  longitude decimal
```

### Notifications

```
notification_topics
  id uuid PK
  name text UNIQUE NOT NULL            -- booking_success | booking_changed | expiry_approaching | arrears_reminder

notification_subscriptions
  user_id uuid FK users
  topic_id uuid FK notification_topics
  PRIMARY KEY (user_id, topic_id)

user_dnd_settings
  user_id uuid PK FK users
  start_time time NOT NULL             -- e.g. 22:00
  end_time time NOT NULL               -- e.g. 06:00
  enabled bool DEFAULT true

notifications
  id uuid PK
  user_id uuid FK users
  topic_id uuid FK notification_topics
  title text NOT NULL
  body text NOT NULL
  read bool DEFAULT false
  dismissed bool DEFAULT false
  created_at timestamptz

notification_jobs
  id uuid PK
  notification_id uuid FK notifications
  status text DEFAULT 'pending'        -- pending | processing | delivered | failed | suppressed | deferred
  attempt_count int DEFAULT 0
  next_attempt_at timestamptz
  last_error text
  booking_id uuid                      -- for frequency cap tracking
  created_at timestamptz

notification_export_packages
  id uuid PK
  channel text NOT NULL                -- sms | email
  recipient text NOT NULL
  body text NOT NULL
  created_at timestamptz
  downloaded_at timestamptz
```

### Campaigns and Tasks

```
campaigns
  id uuid PK
  title text NOT NULL
  description text
  target_role text                     -- nullable = all operators
  created_by uuid FK users
  created_at timestamptz

tasks
  id uuid PK
  campaign_id uuid FK campaigns
  description text NOT NULL
  deadline timestamptz
  reminder_interval_minutes int DEFAULT 60
  completed_at timestamptz
  completed_by uuid FK users
  created_at timestamptz
```

### Segmentation

```
tags
  id uuid PK
  name text UNIQUE NOT NULL

member_tags
  member_id uuid FK members
  tag_id uuid FK tags
  assigned_at timestamptz
  assigned_by uuid FK users
  PRIMARY KEY (member_id, tag_id)

segment_definitions
  id uuid PK
  name text NOT NULL
  filter_expression jsonb NOT NULL     -- structured filter: tags + attribute conditions
  schedule text DEFAULT 'manual'       -- manual | nightly
  created_by uuid FK users
  created_at timestamptz

segment_runs
  id uuid PK
  segment_id uuid FK segment_definitions
  ran_at timestamptz NOT NULL
  member_count int NOT NULL
  triggered_by text                    -- manual | scheduler

tag_versions
  id uuid PK
  exported_by uuid FK users
  exported_at timestamptz NOT NULL
  snapshot jsonb NOT NULL              -- {member_id: [tag_names]}
  imported_at timestamptz
  imported_by uuid FK users
```

### Analytics and Exports

```
exports
  id uuid PK
  requested_by uuid FK users
  format text NOT NULL                 -- csv | excel | pdf
  scope text NOT NULL                  -- occupancy | bookings | exceptions
  segment_id uuid FK segment_definitions  -- nullable
  status text DEFAULT 'pending'        -- pending | ready | failed
  file_path text
  created_at timestamptz
  completed_at timestamptz
```

### Audit

```
audit_logs
  id uuid PK
  actor_id uuid FK users
  action text NOT NULL
  resource_type text
  resource_id uuid
  detail jsonb
  created_at timestamptz NOT NULL
  -- NO UPDATE, NO DELETE permissions for app DB role
```

---

## 6. Key Flows

### Reservation Create → Confirm

```
1. Client sends POST /api/reservations/hold
2. Handler validates input, calls reservations.Service.CreateHold()
3. Service opens DB transaction
4. SELECT FOR UPDATE on capacity_buckets WHERE zone_id + time_window
5. Check available_stalls >= requested stall_count → else 409
6. INSERT capacity_holds (expires_at = now + zone.hold_timeout_minutes)
7. UPDATE capacity_buckets SET available_stalls -= stall_count
8. INSERT reservations (status = 'hold')
9. INSERT booking_events (hold_created)
10. Commit transaction
11. Return 201 + hold_id

12. Client sends POST /api/reservations/:id/confirm
13. Service opens DB transaction
14. SELECT reservation WHERE id AND status = 'hold' FOR UPDATE
15. Check hold not expired → else 409
16. UPDATE reservations SET status = 'confirmed'
17. UPDATE capacity_holds SET released_at = now
18. INSERT booking_events (confirmed)
19. Commit transaction
20. Trigger notification job (booking_success)
```

### Capacity Reconciliation (every 30 min)

```
1. Scheduler fires reconciliation job
2. For each zone:
   a. Sum event-derived occupancy from device_events
   b. Load latest capacity_snapshot for zone
   c. Compare: delta = event_derived - snapshot_authoritative
   d. If delta != 0:
      - INSERT compensating_events (delta)
      - UPDATE capacity_buckets to correct available_stalls
3. INSERT reconciliation_runs (zones_checked, discrepancies_found)
4. INSERT audit_logs (action = 'reconciliation.run')
```

### Notification Dispatch

```
1. Trigger event fires (e.g. reservation confirmed)
2. rules.Evaluate() finds matching active message_rules
3. For each matching rule:
   a. Find subscribed users for topic
   b. For each user:
      - Check frequency cap (3 reminders/booking/day) → skip if exceeded
      - INSERT notifications
      - INSERT notification_jobs (status = 'pending')
4. Dispatcher worker polls notification_jobs WHERE status = 'pending' AND next_attempt_at <= now
5. For each job:
   a. Check user DND schedule → if active, UPDATE status = 'deferred', set next_attempt_at = DND end
   b. Deliver in-app (UPDATE notifications, UPDATE job status = 'delivered')
   c. On failure: exponential backoff, increment attempt_count
   d. After 5 attempts: UPDATE status = 'failed'
```

### Device Event Ingestion

```
1. Device sends POST /api/device-events
2. Validate: device_id, sequence_number, event_key all present → else 400
3. Check event_key in device_events → if exists, return 200 (idempotent)
4. Check sequence_number vs last processed for device:
   - If within 10-min window and out of order → buffer and reorder
   - If outside 10-min window → store with late = true
5. INSERT device_events
6. Process event effects (update occupancy projections)
7. Check for exception conditions → INSERT exceptions if triggered
```

---

## 7. Security Design

### Authentication
- Username + password only. No OAuth, no external IdP.
- Passwords hashed with argon2id (memory=64MB, iterations=3, parallelism=2).
- Minimum 12 characters, must contain letters and digits.
- Failed login counter per user. After 5 consecutive failures: set `locked_until = now + 15 minutes`.
- Session stored server-side in `sessions` table. Token is a random UUID in a cookie (HttpOnly, SameSite=Strict).
- Inactivity timeout: session middleware checks `last_active_at`. If `now - last_active_at > 30 minutes`, invalidate session.
- `force_password_change` flag: set by admin on password reset. Middleware blocks all routes except `PATCH /api/me/password` until cleared.

### Authorization
- RBAC middleware runs on every route before the handler.
- Route groups are protected by role: admin routes, auditor routes, dispatch routes, fleet routes.
- Object-level authorization: handlers verify resource ownership before read/write (e.g. Fleet Manager can only access their org's vehicles).

### Sensitive Field Encryption
- Fields: `member.contact_notes`, `users.api_tokens` (future).
- Algorithm: AES-256-GCM. Envelope: `[12-byte IV][ciphertext][16-byte auth tag]`, base64url encoded.
- Encryption key loaded from `ENCRYPTION_KEY` env var (32 bytes hex). Never hardcoded.
- Raw values never stored, never returned in API responses, never logged.

### Audit Log
- Append-only. PostgreSQL app role has INSERT only on `audit_logs` — no UPDATE, no DELETE.
- Every security-sensitive action writes an audit entry: login, logout, lockout, role change, password reset, tag import, export, device replay, RBAC denial.

---

## 8. Background Jobs

All jobs run as goroutines inside the main process. The scheduler uses a simple ticker loop. Jobs are DB-backed — state survives restarts.

| Job | Interval | Description |
|---|---|---|
| Hold expiry | every 1 min | Release expired capacity holds |
| Notification dispatcher | every 5 sec | Process pending notification jobs |
| Reconciliation | every 30 min | Compare event-derived counts vs snapshots |
| Segment nightly | 02:00 daily | Evaluate all nightly-scheduled segments |
| Task reminder | every 1 min | Generate reminders for incomplete tasks past deadline |

---

## 9. Logging

Using `slog` (Go stdlib). Structured JSON output in production, text in development.

Log levels:
- `INFO` — request in/out, job start/complete, auth events
- `WARN` — failed login attempt, DND suppression, frequency cap hit
- `ERROR` — DB errors, job failures, unexpected states

Never log: passwords, password hashes, session tokens, encryption keys, raw sensitive field values.

Every request log includes: method, path, status, duration, user_id (if authenticated).

---

## 10. Error Handling

All API errors return:
```json
{
  "code": "ERROR_CODE",
  "message": "human readable message"
}
```

Standard codes:
- `VALIDATION_ERROR` — 400
- `UNAUTHORIZED` — 401
- `FORBIDDEN` — 403
- `NOT_FOUND` — 404
- `CONFLICT` — 409 (oversell, duplicate, invalid state transition)
- `RATE_LIMITED` — 429 (lockout)
- `INTERNAL_ERROR` — 500 (never exposes stack trace)

UI pages: all API failures show an inline error message. No white screens.

---

## 11. Docker Setup

```yaml
# docker-compose.yml
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://parkops:parkops@db:5432/parkops
      - ENCRYPTION_KEY=${ENCRYPTION_KEY}
      - SESSION_SECRET=${SESSION_SECRET}
    depends_on:
      db:
        condition: service_healthy

  db:
    image: postgres:16
    environment:
      POSTGRES_USER: parkops
      POSTGRES_PASSWORD: parkops
      POSTGRES_DB: parkops
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U parkops"]
      interval: 5s
      timeout: 5s
      retries: 5
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

On startup, `db/migrate.go` runs all pending migrations before the HTTP server accepts requests.

---

## 12. UI Design Approach

Templ renders full pages server-side. Live updates use HTMX polling (`hx-trigger="every 10s"`) on the activity feed and capacity dashboard — no full page reload needed.

Tailwind CSS via CDN for styling. No frontend build step.

Pages:
- `/login` — unauthenticated layout
- `/dashboard` — operator console (activity feed + capacity cards)
- `/reservations` — calendar + create modal
- `/capacity` — zone stall counts by time window
- `/notifications` — notification center
- `/campaigns` — campaign + task list
- `/segments` — segment builder + preview
- `/analytics` — pivot charts + export
- `/devices` — device list + event log
- `/exceptions` — open exceptions + acknowledge
- `/audit` — audit log viewer (Auditor + Admin only)
- `/admin/users` — user management (Admin only)

All pages show a consistent nav sidebar with role-based menu items — links not permitted for the current role are hidden.

---

## 13. Known Risk Mitigations (Lessons from Task 73)

These are specific issues that caused a partial pass on a similar project. Each one is explicitly addressed in ParkOps.

---

### Risk 1 — Admin endpoint exposes raw password reset tokens

**Task 73 issue:** The admin endpoint returned the raw reset token in the API response, creating a high-risk exposure if the endpoint was ever accessed by an unauthorized party.

**ParkOps fix:**
- `POST /api/admin/users/:id/reset-password` does NOT return the token in the response.
- The admin sets a new temporary password directly. The user logs in with that password.
- `force_password_change = true` is set on the user record.
- No token is generated, stored, or transmitted. There is nothing to expose.
- The response only returns `{"message": "password reset, user must change on next login"}`.
- This is enforced in `auth/service.go` — the reset function takes a plaintext password, hashes it, stores the hash, and sets the flag. No token involved.

---

### Risk 2 — CIDR/IP restrictions configured but not enforced

**Task 73 issue:** Webhook CIDR restrictions were declared in config but the actual enforcement code was missing, so any URL was accepted regardless of the configured restriction.

**ParkOps equivalent:** Device event ingestion and the local network constraint.

**ParkOps fix:**
- The system is offline/local-only. There are no outbound webhook calls.
- For device event ingestion, the `POST /api/device-events` endpoint validates that the request includes a valid `device_id` that exists in the `devices` table. Unregistered devices are rejected with 403.
- If IP-based restrictions are added in future, they must be enforced in middleware with a test that actually sends a request from a disallowed IP and verifies rejection — not just a config check.
- Rule: any security constraint declared in config MUST have a corresponding test that verifies enforcement, not just existence.

---

### Risk 3 — Frontend UI workflows under-tested

**Task 73 issue:** Test coverage was weak on the frontend side, leaving UI workflows and edge-case interactions untested.

**ParkOps fix:**
- ParkOps uses Templ (SSR) — there is no separate frontend framework. UI is rendered server-side.
- API tests cover the full request/response cycle including the data that drives the UI.
- For each Templ page, the corresponding API endpoint is tested for: happy path, error state, empty state, and permission-denied state.
- The `run_tests.sh` script runs both unit tests and API tests. A page is not considered done until its API tests pass all four scenarios above.
- Specific UI edge cases to test via API:
  - Capacity dashboard with zero stalls available
  - Activity feed with no recent events (empty state)
  - Notification center with no unread notifications
  - Segment preview returning zero members
  - Export with no data in the selected range

---

### Risk 4 — Dynamic regex rules lack ReDoS safety guards

**Task 73 issue:** Regex content rules could be configured with patterns that cause catastrophic backtracking (ReDoS), hanging the server under load.

**ParkOps equivalent:** Segment filter expressions use filter logic, not raw regex. Message rules use template strings, not regex.

**ParkOps fix:**
- Segment filter expressions are structured JSON (e.g. `{"and": [{"tag": "downtown"}, {"arrears_balance_cents": {"gt": 5000}}]}`), not raw regex. No ReDoS risk.
- Message rule templates are plain text with variable substitution (e.g. `"Hello {{name}}, your booking expires soon"`). No regex evaluation.
- If regex is ever added (e.g. for content filtering on listing titles), it MUST:
  1. Be validated against a ReDoS detector before storage (e.g. `regexp.MustCompile` with a timeout context in Go)
  2. Have a maximum pattern length limit (e.g. 200 chars)
  3. Be tested with a known ReDoS pattern (e.g. `(a+)+$`) to verify it is rejected or times out safely
- Rule: never evaluate user-supplied regex directly against input without a timeout wrapper.

---

### Summary — Pre-build Security Checklist

Before submitting, verify these four items explicitly:

- [ ] Admin password reset endpoint returns NO token — only sets force_password_change flag
- [ ] Every security constraint in config has a test that verifies enforcement (not just existence)
- [ ] Every UI page has API tests for: happy path, error state, empty state, 403 state
- [ ] No user-supplied regex is evaluated without a timeout and length guard
