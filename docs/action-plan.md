# ParkOps — Action Plan

## MVP Scope

Build these 6 slices first:
- Authentication + RBAC
- Lots / zones / rate plans
- Reservations + capacity hold
- Operator dashboard
- Notifications (in-app only)
- Audit log

Leave for later:
- Device replay correction
- Drift smoothing for live tracking
- Segmentation import/export rollback
- PDF/Excel export
- Advanced analytics pivots
- SMS/email exportable message packages

---

## Phase 0 — Project Setup

Tasks:
- Initialize Go module
- Set up Gin
- Set up Templ
- Connect PostgreSQL
- Config loading from env
- Add migrations runner
- Structured logging
- Basic error handling / request middleware
- Session auth middleware
- Seed script for admin user

Deliverables:
- App boots locally
- Login page works
- Protected routes work
- Database migrations run

---

## Phase 1 — Auth and RBAC

Entities: `users`, `roles`, `permissions`, `user_roles`, `sessions`, `audit_logs`

Tasks:
- Username/password login (min 12 characters enforced)
- Password hashing (bcrypt or argon2)
- Failed-login counter per user
- 15-minute lockout after 5 failed attempts
- 30-minute inactivity session timeout
- Sensitive fields encrypted at rest (password hashes, API tokens, member contact notes)
- Middleware for role checks on every route
- Tamper-evident audit log entries for login, logout, lockout, role changes

Pages: login, user management, role/permission editor, session/logout, admin password reset

API:
```
POST  /api/auth/login
POST  /api/auth/logout
GET   /api/me
PATCH /api/me/password                        # change own password
POST  /api/admin/users/:id/reset-password     # admin resets another user's password (offline — no email)
GET   /api/admin/users
POST  /api/admin/users
PATCH /api/admin/users/:id
DELETE /api/admin/users/:id
PATCH /api/admin/users/:id/roles
POST  /api/admin/users/:id/unlock             # manually unlock a locked account
GET   /api/admin/users/:id/sessions           # view active sessions for a user
DELETE /api/admin/users/:id/sessions          # force-expire all sessions for a user
GET   /api/admin/audit-logs                   # audit log viewer (Auditor + Admin)
```

Note on password reset: this system is offline/local-only — there is no email-based forgot password flow. Password resets are admin-initiated only. An admin sets a temporary password for the user; the user is forced to change it on next login. Add a `force_password_change` flag to the users table.

---

## Phase 2 — Master Data

Entities: `facilities`, `lots`, `zones`, `rate_plans`, `message_rules`, `members`, `vehicles`, `drivers`

Tasks:
- CRUD for facilities, lots, zones
- Capacity per zone (total stalls)
- Hold timeout per zone (default 15 min, overridable)
- Rate plan definitions (linked to zone or lot)
- Member management (Fleet Manager scope)
- Vehicle and driver management
- Message rule definitions (used by notification trigger rules)
- Admin validation rules

Pages: lots list/detail, zones list/detail, rate plans, members, vehicles/drivers

API:
```
GET/POST        /api/facilities
GET/PATCH/DELETE /api/facilities/:id
GET/POST        /api/lots
GET/PATCH/DELETE /api/lots/:id
GET/POST        /api/zones
GET/PATCH/DELETE /api/zones/:id
GET/POST        /api/rate-plans
GET/PATCH/DELETE /api/rate-plans/:id
GET/POST        /api/members
GET/PATCH/DELETE /api/members/:id
GET/POST        /api/vehicles
GET/PATCH/DELETE /api/vehicles/:id
GET/POST        /api/drivers
GET/PATCH/DELETE /api/drivers/:id
GET/POST        /api/message-rules
GET/PATCH/DELETE /api/message-rules/:id
```

---

## Phase 3 — Reservations and Capacity Engine

Entities: `reservations`, `reservation_items`, `capacity_holds`, `capacity_buckets`, `booking_events`, `capacity_snapshots`

Rules:
- Create reservation → atomic hold (row-level lock on capacity bucket)
- Capacity bucket keyed by `zone_id + time_window_start + time_window_end`
- Default hold expires in 15 minutes (zone-configurable)
- Confirm reservation → decrement real capacity + release hold
- Cancel/expire → release hold + restore stall count
- No oversell — reject if no stalls available

Tasks:
- Availability search by zone + time window
- Create hold in transaction
- Reservation confirmation
- Cancellation
- Hold expiration job
- Conflict warning before confirmation
- Remaining stalls by zone/time window endpoint

Pages: reservation calendar, create reservation modal, conflict warning UI, booking detail and timeline

API:
```
GET  /api/availability
POST /api/reservations/hold
POST /api/reservations
POST /api/reservations/:id/confirm
POST /api/reservations/:id/cancel
GET  /api/reservations/:id
GET  /api/reservations
PATCH /api/reservations/:id              # update reservation details (before confirmation)
GET  /api/capacity/dashboard
GET  /api/capacity/zones/:id/stalls      # remaining stalls for a zone + time window
GET  /api/reservations/:id/timeline      # booking event history
```

---

## Phase 4 — Operator Console

Widgets: live activity feed, arrivals/departures, exceptions, zone capacity cards, reservation calendar, current alerts, upcoming expirations

Tasks:
- Build dashboard layout with Templ
- Poll every 10–15 seconds for live updates (SSE upgrade later)
- Show remaining stalls per zone for selected time window
- Show upcoming hold expirations
- Show over-capacity warnings
- Show recent device events and gate exceptions
- Dispatch Operator exception acknowledgement

API:
```
GET  /api/exceptions                        # list open exceptions
GET  /api/exceptions/:id
POST /api/exceptions/:id/acknowledge        # Dispatch Operator acknowledges with optional note
GET  /api/exceptions/history                # acknowledged/closed exceptions
```

---

## Phase 5 — Notifications and Tasks (Campaigns)

Entities: `notification_topics`, `notification_subscriptions`, `notifications`, `notification_jobs`, `user_dnd_settings`, `campaigns`, `tasks`, `task_assignments`

Tasks:
- Subscribe/unsubscribe to topics (booking success, changes, expiry, arrears)
- Trigger rule engine — generate notification job on matching event
- In-app delivery (immediate)
- DND schedule enforcement (defer until DND end time)
- Frequency cap enforcement (max 3 reminders per booking per day)
- Suppression log for blocked notifications
- Retry failed sends with exponential backoff (max 5 attempts)
- Campaign create/edit with one or more tasks
- Task create with description + optional deadline
- Task reminder jobs until marked complete
- Mark task complete endpoint

Pages: notification center, task/campaign center, user notification settings

API:
```
GET  /api/notifications
GET  /api/notifications/:id
PATCH /api/notifications/:id/read
POST /api/notifications/:id/dismiss
GET  /api/notification-settings
PATCH /api/notification-settings
GET  /api/notification-settings/dnd
PATCH /api/notification-settings/dnd
GET  /api/notification-topics
POST /api/notification-topics/:id/subscribe
DELETE /api/notification-topics/:id/subscribe
GET  /api/campaigns
POST /api/campaigns
GET  /api/campaigns/:id
PATCH /api/campaigns/:id
DELETE /api/campaigns/:id
GET  /api/campaigns/:id/tasks
POST /api/campaigns/:id/tasks
GET  /api/tasks/:id
PATCH /api/tasks/:id
POST /api/tasks/:id/complete
DELETE /api/tasks/:id
```

---

## Phase 6 — Device Event Ingestion

Entities: `devices`, `device_events`, `event_replays`, `vehicle_positions`

Tasks:
- Device registration
- Inbound event API — validate device ID, sequence number, unique event key
- Idempotency layer — deduplicate by event key
- Out-of-order correction within 10-minute window
- Late event flagging (outside 10-minute window)
- Controlled replay without double-counting
- Offline buffer and retransmission support
- Real-time location report ingestion
- Stop detection (stationary > 3 minutes)
- Trusted timestamp recording (server time + signed device time)
- Drift smoothing (threshold-based, configurable distance/time window)

API:
```
POST /api/device-events
POST /api/device-events/replay
GET  /api/device-events                  # list with filters (device, time range, late flag)
GET  /api/device-events/:id
GET  /api/devices
POST /api/devices
GET  /api/devices/:id
PATCH /api/devices/:id
DELETE /api/devices/:id
POST /api/tracking/location
GET  /api/tracking/vehicles/:id/positions  # position history
GET  /api/tracking/vehicles/:id/stops      # stop events
```

---

## Phase 7 — Reconciliation Jobs

Entities: `reconciliation_runs`, `compensating_events`

Tasks:
- Scheduled reconciliation worker (every 30 minutes)
- Compare event-derived occupancy vs authoritative capacity snapshots per zone
- Detect any non-zero discrepancy
- Generate compensating release or hold records
- Audit log entry for every reconciliation run and correction

---

## Phase 8 — Tagging and Segmentation

Entities: `tags`, `member_tags`, `segment_definitions`, `segment_runs`, `tag_versions`

Tasks:
- Add/remove tags on member records
- Segment definition builder (filter expressions over tags + attributes)
- Segment preview — count matching members before activation
- On-demand segment evaluation
- Nightly scheduled segment evaluation job
- Tag version export (JSON snapshot)
- Tag version import (restore previous snapshot)
- Audit log entry on every import

API:
```
GET  /api/tags
POST /api/tags
DELETE /api/tags/:id
POST /api/members/:id/tags               # add tag to member
DELETE /api/members/:id/tags/:tagId      # remove tag from member
GET  /api/segments
POST /api/segments
GET  /api/segments/:id
PATCH /api/segments/:id
DELETE /api/segments/:id
POST /api/segments/:id/preview           # returns matching member count
POST /api/segments/:id/run               # on-demand evaluation
GET  /api/segments/:id/runs              # run history
GET  /api/members/:id/tags               # tags for a member
POST /api/tags/export                    # export tag version snapshot
POST /api/tags/import                    # import and restore tag version
```

---

## Phase 9 — Analytics and Exports

Tasks:
- Occupancy trend charts
- Booking distribution pivots by time, region, category, entity, risk level
- CSV export (first)
- Role-restricted sharing enforcement
- Segment-restricted dataset visibility
- On-screen trend and distribution charts
- PDF/Excel export (later)
- SMS/email notification export packages download
- Capacity snapshot history endpoint

API:
```
GET  /api/analytics/occupancy
GET  /api/analytics/bookings
GET  /api/analytics/exceptions
GET  /api/exports
POST /api/exports
GET  /api/exports/:id/download
GET  /api/notifications/export-packages
GET  /api/notifications/export-packages/:id/download
GET  /api/capacity/snapshots
GET  /api/members/:id/balance
PATCH /api/members/:id/balance           # admin manual arrears adjustment
```

---

## Project Structure

```
parkops/
├── cmd/web/main.go
├── internal/
│   ├── app/            # app.go, config.go, router.go
│   ├── auth/           # handler, service, repo, model, password
│   ├── users/          # handler, service, repo, model
│   ├── rbac/           # middleware, service, model
│   ├── facilities/
│   ├── zones/
│   ├── rates/
│   ├── members/
│   ├── vehicles/
│   ├── reservations/   # hold_engine.go, calendar.go
│   ├── capacity/       # reconciliation.go
│   ├── devices/        # ingest.go, dedupe.go
│   ├── tracking/       # smoother.go, stop_detector.go
│   ├── notifications/  # dispatcher.go, rules.go
│   ├── campaigns/
│   ├── tasks/
│   ├── tags/
│   ├── segments/       # runner.go
│   ├── analytics/
│   ├── audit/
│   ├── jobs/           # worker.go, scheduler.go, registry.go
│   ├── db/             # postgres.go, migrations/, tx.go
│   ├── web/
│   │   ├── handlers/
│   │   ├── middleware/
│   │   ├── templates/  # layouts/, pages/, partials/, components/
│   │   └── static/     # css/, js/, img/
│   └── platform/       # logger/, clock/, security/, pagination/, validator/
├── migrations/
│   ├── 0001_init.sql
│   ├── 0002_auth.sql
│   ├── 0003_master_data.sql
│   ├── 0004_reservations.sql
│   ├── 0005_notifications.sql
│   ├── 0006_devices.sql
│   ├── 0007_tags_segments.sql
│   └── 0008_analytics.sql
├── scripts/            # dev.sh, seed.sh, test.sh
├── docs/
├── .env.example
├── go.mod
├── Makefile
└── Dockerfile
```

---

## Database Tables

### MVP — build first

| Group | Tables |
|-------|--------|
| Auth | `users`, `roles`, `permissions`, `user_roles`, `sessions` |
| Master data | `facilities`, `lots`, `zones`, `rate_plans`, `members` (includes `arrears_balance_cents`), `vehicles`, `drivers`, `message_rules` |
| Reservations | `reservations`, `reservation_items`, `capacity_holds`, `capacity_buckets`, `booking_events` |
| Notifications | `notification_topics`, `notification_subscriptions`, `notifications`, `notification_jobs`, `user_dnd_settings`, `notification_export_packages` |
| Campaigns | `campaigns`, `tasks`, `task_assignments` |
| Exceptions | `exceptions` |
| Audit | `audit_logs` |

### Add later

| Group | Tables |
|-------|--------|
| Devices | `devices`, `device_events`, `event_replays`, `vehicle_positions` |
| Capacity | `capacity_snapshots`, `reconciliation_runs`, `compensating_events` |
| Segmentation | `tags`, `member_tags`, `segment_definitions`, `segment_runs`, `tag_versions` |
| Analytics | materialized views |

---

## Tech Stack

| Layer | Choice |
|-------|--------|
| HTTP | Gin |
| UI | Templ (SSR) |
| DB access | pgx + sqlc |
| Migrations | goose or golang-migrate |
| Logging | slog or zap |
| Encryption | AES-256-GCM for sensitive fields at rest |

---

## MVP Done When

- Login/logout works with lockout and inactivity timeout
- Admin can create facility/lot/zone/rate plan
- Operator can create reservations
- System prevents oversell atomically
- Dashboard shows remaining stalls per zone
- Reservation cancel releases capacity
- Audit logs are written and append-only
- In-app notifications appear with DND and frequency cap respected
- Campaigns and tasks generate reminders until complete

---

## Critical Implementation Notes (from AI Self Test)

These are hard constraints that must be correct — not just present:

**Capacity hold atomicity**
Use `SELECT FOR UPDATE` or `SERIALIZABLE` transaction isolation on the capacity bucket row. Two concurrent requests for the last stall must result in exactly one success and one 409. Test this with concurrent requests.

**Confirm re-checks availability**
The confirm endpoint must re-check stall availability after the hold. If the hold expired between creation and confirmation, return a conflict error — do not silently confirm against a stale hold.

**Notification DND defers, not drops**
When a notification job is ready and DND is active, reschedule the job for the DND end time. Do not delete or skip it. The job must survive a server restart (persisted in PostgreSQL).

**Frequency cap check before job creation**
Check the frequency cap before inserting a notification job into the queue. Do not create the job and then suppress it — reject it at the source.

**Device event replay idempotency**
The replay endpoint must check the idempotency store (processed event keys) before applying any event. A replayed event key that was already processed must not increment capacity counts again.

**force_password_change middleware**
After an admin resets a password, set `force_password_change = true` on the user. Add middleware that checks this flag on every protected route and redirects to the password change endpoint until the flag is cleared.

**Audit log DB-level protection**
The application DB role must not have UPDATE or DELETE permissions on the `audit_logs` table. This must be enforced at the PostgreSQL role level, not just at the application layer.

**Sensitive field encryption**
Use AES-256-GCM (or equivalent authenticated encryption). Store IV + ciphertext + auth tag. Never store raw values. Never include raw values in API responses or logs.

**Reconciliation handles both deltas**
The reconciliation job must handle both positive discrepancies (event-derived count too high → compensating release) and negative discrepancies (event-derived count too low → compensating hold). Both directions must be tested.

**Fleet Manager org isolation**
All vehicle, driver, and member queries must filter by `organization_id` derived from the authenticated user's account. This must be enforced in the repository layer, not just the handler.

**Decisions still needed before building**
- What happens when a zone's stall count is reduced below current confirmed reservations?
- Can operators replay device events by time range, or only by individual event key?
- Is there a maximum reminder count for campaign tasks, or do they run until marked complete indefinitely?
- See `docs/questions.md` for the full list of open decisions.
