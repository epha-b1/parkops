# ParkOps — Feature Build Order

Build one slice at a time. Each slice must be fully working (implementation + tests + UI) before moving to the next.

---

## Slice 1 — Project Foundation
What: app boots, DB connects, migrations run, login page renders
Done when:
- `docker compose up` starts cleanly
- migrations run on startup
- `/login` page renders
- structured logging works
- `.env.example` has all required vars

---

## Slice 2 — Authentication
What: login, logout, session, lockout, password rules, force_password_change, admin user management
Done when:
- POST /api/auth/login works with real password check
- wrong password increments fail counter
- 5 fails → account locked for 15 min → 429
- POST /api/admin/users/:id/unlock manually unlocks account
- session cookie set (HttpOnly, SameSite=Strict)
- GET /api/me returns current user
- session expires after 30 min inactivity
- GET /api/admin/users/:id/sessions lists active sessions
- DELETE /api/admin/users/:id/sessions force-expires all sessions
- PATCH /api/me/password enforces 12-char min
- admin reset sets force_password_change flag
- force_password_change blocks all routes except password change
- unit tests: password length, lockout, session timeout, force_change
- API tests: login, wrong password, lockout, logout, me, unlock, force-expire sessions

---

## Slice 3 — RBAC
What: role middleware on every route, 403 on wrong role
Done when:
- every route group has role middleware
- wrong role → 403 + audit log entry
- PATCH /api/admin/users/:id/roles works
- unit tests: role check logic
- API tests: each role against forbidden endpoints

---

## Slice 4 — Master Data (Facilities, Lots, Zones, Rate Plans)
What: admin can create and manage the parking infrastructure
Done when:
- CRUD for facilities, lots, zones, rate plans
- zone has total_stalls and hold_timeout_minutes
- rate plan linked to zone
- Facility Administrator role required
- pages: facilities list, lots list, zones list, rate plans list
- API tests: CRUD happy path + 403 for wrong role

---

## Slice 5 — Members, Vehicles, Drivers
What: Fleet Manager manages their org's people and vehicles
Done when:
- CRUD for members, vehicles, drivers
- all queries org-scoped (Fleet Manager cannot see other orgs)
- arrears_balance_cents on member
- GET/PATCH /api/members/:id/balance (Admin only)
- pages: members list, vehicles list, drivers list
- API tests: CRUD + cross-org 403

---

## Slice 6 — Reservations and Capacity Engine
What: the core of the system — atomic hold, confirm, cancel, oversell prevention
Done when:
- POST /api/reservations/hold creates atomic hold (SELECT FOR UPDATE)
- hold expires after zone.hold_timeout_minutes
- POST /api/reservations/:id/confirm decrements stall count
- confirm fails if hold expired → 409
- POST /api/reservations/:id/cancel releases hold + restores stalls
- GET /api/availability returns remaining stalls
- GET /api/capacity/dashboard shows all zones
- GET /api/capacity/zones/:id/stalls returns stalls for time window
- GET /api/capacity/snapshots lists capacity snapshots
- GET /api/reservations/:id/timeline shows booking event history
- conflict warning shown in UI before confirmation
- reservation calendar page renders
- zone stall count reduction blocked if it would go below confirmed reservations
- unit tests: hold atomicity, expiry, confirm re-check, oversell
- API tests: full flow + concurrent oversell test + expired hold confirm

---

## Slice 7 — Operator Console (Dashboard)
What: live activity feed, capacity cards, exception list
Done when:
- dashboard page renders with zone capacity cards
- activity feed polls every 10s (HTMX hx-trigger)
- upcoming hold expirations shown
- over-capacity warnings shown
- exceptions list shows open exceptions

---

## Slice 8 — Device Integration
What: cameras, gates, sensors send events; idempotency; out-of-order; replay
Done when:
- POST /api/device-events validates device_id + sequence_number + event_key
- duplicate event_key → 200 idempotent (not reprocessed)
- out-of-order events within 10-min window reordered
- late events (>10 min) stored with late=true flag
- POST /api/device-events/replay skips already-processed keys
- device registration works
- unit tests: deduplication, out-of-order, late flag, replay
- API tests: ingest, duplicate, missing fields, replay

---

## Slice 9 — Exceptions
What: device exceptions created, Dispatch Operator acknowledges
Done when:
- exceptions created from device events
- GET /api/exceptions lists open exceptions
- POST /api/exceptions/:id/acknowledge transitions to acknowledged
- acknowledged exceptions removed from active feed
- exception list shown on dashboard
- API tests: list, acknowledge, wrong role 403

---

## Slice 10 — Real-time Tracking
What: location reports, drift smoothing, stop detection
Done when:
- POST /api/tracking/location accepts reports
- drift smoothing marks suspect positions
- stop event created after 3 min stationary
- trusted timestamp recorded (server + signed device time)
- GET /api/tracking/vehicles/:id/positions works
- GET /api/tracking/vehicles/:id/stops works
- unit tests: drift threshold, stop detection at 3 min, timestamp trust

---

## Slice 11 — Reconciliation Job
What: 30-min job corrects capacity drift from late device events
Done when:
- job runs every 30 minutes
- compares event-derived counts vs capacity_snapshots
- generates compensating_events on discrepancy
- audit log entry per run
- unit tests: positive delta, negative delta, no delta

---

## Slice 12 — Notifications
What: trigger rules, queue, DND, frequency cap, retry, in-app delivery
Done when:
- message rules evaluated on booking events
- notification jobs created for subscribed users
- in-app delivery immediate
- DND defers job (not drops) to DND end time
- frequency cap: 4th reminder same booking same day suppressed before job creation
- exponential backoff retry up to 5 attempts
- notification center page renders
- GET /api/notifications lists notifications
- GET /api/notifications/:id gets single notification
- PATCH /api/notifications/:id/read marks read
- POST /api/notifications/:id/dismiss dismisses
- GET /api/notification-topics lists topics with subscribed flag
- POST/DELETE /api/notification-topics/:id/subscribe works
- GET /api/notification-settings returns current settings
- PATCH /api/notification-settings updates settings
- GET/PATCH /api/notification-settings/dnd works
- DND settings page works
- SMS/email export packages generated
- GET /api/notifications/export-packages lists packages
- GET /api/notifications/export-packages/:id/download downloads package
- notification jobs survive server restart (persisted in PostgreSQL)
- unit tests: DND defer, frequency cap, retry backoff
- API tests: subscribe, list, mark read, dismiss, DND settings, export packages

---

## Slice 13 — Campaigns and Tasks
What: campaigns with tasks, deadline reminders until complete
Done when:
- campaign CRUD works
- task CRUD with deadline and reminder interval
- reminder jobs generated until task marked complete
- POST /api/tasks/:id/complete stops reminders
- campaigns page renders
- API tests: create campaign, create task, complete task, reminder stops

---

## Slice 14 — Tagging and Segmentation
What: tag members, define segments, preview, run, export/import
Done when:
- tags CRUD works
- add/remove tags on members
- segment definition with filter expression
- POST /api/segments/:id/preview returns count before activation
- on-demand segment run works
- nightly scheduler evaluates segments at 02:00
- tag version export produces JSON snapshot
- tag version import restores tags + writes audit log
- segments page renders
- unit tests: filter evaluation, preview count, import audit log
- API tests: full segment flow, tag import/export

---

## Slice 15 — Analytics and Exports
What: pivot charts, CSV/Excel/PDF export, role + segment access control
Done when:
- GET /api/analytics/occupancy returns trend data
- GET /api/analytics/bookings returns pivot data
- GET /api/analytics/exceptions returns exception trends
- GET /api/exports lists past exports
- POST /api/exports generates CSV (first)
- GET /api/exports/:id/download downloads file
- export role check enforced → 403 for wrong role
- export segment check enforced → 403 if not in segment
- export audit log entry written
- analytics page renders with charts
- Excel and PDF export (after CSV works)
- API tests: export role 403, segment 403, CSV download, list exports

---

## Slice 16 — Audit Log Viewer
What: Auditor and Admin can query the audit log
Done when:
- GET /api/admin/audit-logs returns filtered results
- Auditor role can read, cannot write/delete
- audit_logs table has no UPDATE/DELETE for app DB role
- audit log page renders
- API tests: read as auditor, 403 for other roles

---

## Slice 17 — Final Polish
What: clean up, full test run, Docker cold start
Done when:
- `run_tests.sh` passes all unit + API tests
- `docker compose up` cold start works
- README has correct URLs, ports, test credentials
- no vendor/, no compiled binaries in repo
- no real credentials in any config file
- screenshots captured
- self-test report completed
- Task 73 failure checks all pass:
  - admin reset endpoint returns NO token in response
  - every security constraint has a test verifying enforcement
  - every UI page tested for happy path + error + empty + 403
  - no user-supplied regex evaluated without timeout guard
