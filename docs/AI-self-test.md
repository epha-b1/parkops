# ParkOps — AI Self Test (Delivery Acceptance / Project Architecture Review)

You are the "Delivery Acceptance / Project Architecture Review" inspector. Conduct item-by-item verification of the project, strictly outputting results based on the acceptance criteria below.

---

## Business/Topic Prompt

Create a ParkOps Command & Reservation platform that lets a parking operator run reservations, capacity, and operational events entirely on an offline local network. Facility Administrators manage lots, zones, rate plans, and message rules; Dispatch Operators monitor live movements and exceptions; Fleet Managers (business members) manage vehicles and drivers; and Auditors review immutable histories and exports. The Templ-based web UI provides an operator console with a real-time activity feed, a reservation calendar, and a capacity dashboard that shows remaining stalls per zone and time window with immediate conflict warnings before confirmation. Users can subscribe to in-app notification topics such as booking success, booking changes, approaching expiration, and arrears reminders, set Do Not Disturb hours (for example, 10:00 PM to 6:00 AM), and control frequency caps (no more than 3 reminders per booking per day). A built-in campaign/task area supports configurable reading and activity tasks that generate reminders until marked complete. Member tagging and segmentation enable targeted operations with preview counts before activation; segments can run on-demand or on a nightly schedule, and operators can export or import tag versions for rollback. Analytics screens provide time/region/category/entity/risk-level pivots, trend and distribution charts, and exports to CSV/Excel/PDF with sharing restricted by role and by segment membership.

On the system side, the backend uses Gin with PostgreSQL as the local system of record. The Inventory/Availability Consistency Engine enforces oversell prevention by taking an atomic capacity hold on create (default hold timeout 15 minutes), decrementing on confirmation, and releasing on cancellation/expiration; a reconciliation job runs every 30 minutes. Parking device integration supports cameras, gates, and geomagnetic sensors with offline buffering; every inbound event must include a device identifier, a monotonic sequence number, and a unique event key for idempotency, out-of-order correction within a 10-minute window, and controlled replay without double-counting. Real-time tracking accepts vehicle/mobile location reports, corrects drift, detects stops longer than 3 minutes, and records trusted timestamps. Notifications use exponential backoff retry (max 5 attempts); in-app delivery is immediate; SMS/email are exportable packages. Security is local-first: password min 12 chars, lockout after 5 fails for 15 minutes, session expiry after 30 minutes inactivity, sensitive fields encrypted at rest, RBAC on every API, tamper-evident audit log.

---

## 1. Mandatory Thresholds

**1.1 Runability**
- Does it provide clear startup instructions (README, Makefile)?
- Can it start without modifying core code?
- Does runtime result match delivery description?

**1.2 Theme Alignment**
- Does delivered content revolve around the parking operations business goal?
- Has the core problem been replaced, weakened, or ignored?

---

## 2. Delivery Completeness Checklist

Verify each item is implemented:

### Roles and Access
- [ ] Facility Administrator: lots, zones, rate plans, message rules CRUD
- [ ] Dispatch Operator: live movements, exception monitoring, acknowledgement
- [ ] Fleet Manager: vehicles and drivers (org-scoped only)
- [ ] Auditor: read-only history and exports

### UI
- [ ] Operator console: real-time activity feed, reservation calendar, capacity dashboard
- [ ] Conflict warning before reservation confirmation
- [ ] Remaining stalls per zone shown for selected time window

### Notifications
- [ ] Topics: booking success, changes, expiry, arrears
- [ ] DND schedule per user (start time, end time)
- [ ] Frequency cap: max 3 reminders per booking per day
- [ ] Notification deferred (not dropped) during DND
- [ ] Exponential backoff retry, max 5 attempts
- [ ] In-app delivery immediate
- [ ] SMS/email as exportable message packages (no external connectivity)

### Campaigns and Tasks
- [ ] Campaign create/edit with one or more tasks
- [ ] Task deadline and reminder until marked complete

### Segmentation
- [ ] Member tagging (add/remove)
- [ ] Segment definition with filter expressions
- [ ] Segment preview count before activation
- [ ] On-demand and nightly segment evaluation
- [ ] Tag version export and import for rollback
- [ ] Audit log entry on tag import

### Analytics
- [ ] Pivots: time, region, category, entity, risk level
- [ ] Trend and distribution charts
- [ ] CSV export
- [ ] Excel and PDF export
- [ ] Role-restricted export sharing
- [ ] Segment-restricted export visibility

### Capacity Engine
- [ ] Atomic capacity hold on reservation create (15-min default, zone-configurable)
- [ ] Decrement on confirmation, release on cancel/expiry
- [ ] Oversell prevention: reject if no stalls available
- [ ] Reconciliation job every 30 minutes
- [ ] Compensating release on discrepancy

### Device Integration
- [ ] Device event ingestion: camera, gate, geomagnetic sensor
- [ ] Required fields: device ID + sequence number + unique event key
- [ ] Idempotency: deduplication by event key
- [ ] Out-of-order correction within 10-minute window
- [ ] Late event flagging (outside window)
- [ ] Controlled replay without double-counting
- [ ] Offline buffering and retransmission

### Tracking
- [ ] Real-time location report ingestion
- [ ] Drift smoothing for sudden GPS jumps
- [ ] Stop detection > 3 minutes
- [ ] Trusted timestamp: server time + signed device time

### Security
- [ ] Password min 12 characters enforced
- [ ] Lockout after 5 failed attempts for 15 minutes
- [ ] Session expiry after 30 minutes inactivity
- [ ] Sensitive fields encrypted at rest (password hashes, API tokens, member contact notes)
- [ ] RBAC middleware on every API endpoint
- [ ] Tamper-evident audit log (append-only, no UPDATE/DELETE for app role)
- [ ] Admin-initiated password reset (no email — offline system)
- [ ] force_password_change flag enforced on next login after admin reset

---

## 3. Engineering Quality Checklist

### Module Structure (verify each exists)
- [ ] `internal/auth/` — login, password, lockout, session
- [ ] `internal/rbac/` — middleware, role checks
- [ ] `internal/users/`
- [ ] `internal/facilities/`, `internal/zones/`, `internal/rates/`
- [ ] `internal/members/`, `internal/vehicles/`, `internal/drivers/`
- [ ] `internal/reservations/` — hold_engine, calendar
- [ ] `internal/capacity/` — reconciliation
- [ ] `internal/devices/` — ingest, dedupe
- [ ] `internal/tracking/` — smoother, stop_detector
- [ ] `internal/notifications/` — dispatcher, rules
- [ ] `internal/campaigns/`, `internal/tasks/`
- [ ] `internal/tags/`, `internal/segments/`
- [ ] `internal/analytics/`
- [ ] `internal/audit/`
- [ ] `internal/jobs/` — worker, scheduler
- [ ] `internal/db/` — postgres, migrations, tx
- [ ] `internal/web/` — handlers, middleware, templates, static
- [ ] `internal/platform/` — logger, clock, security, pagination, validator
- [ ] `migrations/` — numbered SQL files
- [ ] `cmd/web/main.go`
- [ ] `Makefile`, `Dockerfile`, `.env.example`

### Engineering Details
- [ ] Structured logging (slog or zap) — not fmt.Println
- [ ] Request validation on all input endpoints
- [ ] Consistent JSON error response: error code + message
- [ ] DB transactions for atomic operations (capacity hold, reservation confirm)
- [ ] Migrations numbered and sequential
- [ ] Config loaded from env (not hardcoded)
- [ ] Sensitive values never logged

---

## 4. Security Audit Checklist (Priority)

- [ ] `POST /api/auth/login` — password check, lockout enforced
- [ ] Session middleware on every protected route — inactivity timeout checked
- [ ] RBAC middleware applied to every route group
- [ ] Object-level auth: reservation/device ownership checked before read/write
- [ ] `/api/admin/*` routes require admin role
- [ ] Audit log routes require auditor or admin role
- [ ] Fleet Manager cannot see other orgs' vehicles/drivers
- [ ] Export endpoint checks segment membership before generating
- [ ] Only admin can reset another user's password
- [ ] force_password_change blocks all non-password-change routes until changed
- [ ] Sensitive fields: raw values never stored, never in API responses
- [ ] audit_logs table: no UPDATE/DELETE for app DB role
- [ ] Replay endpoint checks idempotency store before applying
- [ ] Admin can force-expire all sessions for a user

---

## 5. API Endpoint Completeness Checklist

### Auth
- [ ] POST /api/auth/login
- [ ] POST /api/auth/logout
- [ ] GET /api/me
- [ ] PATCH /api/me/password
- [ ] POST /api/admin/users/:id/reset-password
- [ ] POST /api/admin/users/:id/unlock
- [ ] GET /api/admin/users/:id/sessions
- [ ] DELETE /api/admin/users/:id/sessions

### Users / Admin
- [ ] GET /api/admin/users
- [ ] POST /api/admin/users
- [ ] PATCH /api/admin/users/:id
- [ ] DELETE /api/admin/users/:id
- [ ] PATCH /api/admin/users/:id/roles
- [ ] GET /api/admin/audit-logs

### Master Data
- [ ] GET/POST /api/facilities, GET/PATCH/DELETE /api/facilities/:id
- [ ] GET/POST /api/lots, GET/PATCH/DELETE /api/lots/:id
- [ ] GET/POST /api/zones, GET/PATCH/DELETE /api/zones/:id
- [ ] GET/POST /api/rate-plans, GET/PATCH/DELETE /api/rate-plans/:id
- [ ] GET/POST /api/members, GET/PATCH/DELETE /api/members/:id
- [ ] GET/POST /api/vehicles, GET/PATCH/DELETE /api/vehicles/:id
- [ ] GET/POST /api/drivers, GET/PATCH/DELETE /api/drivers/:id
- [ ] GET/POST /api/message-rules, GET/PATCH/DELETE /api/message-rules/:id

### Reservations
- [ ] GET /api/availability
- [ ] POST /api/reservations/hold
- [ ] POST /api/reservations
- [ ] POST /api/reservations/:id/confirm
- [ ] POST /api/reservations/:id/cancel
- [ ] GET /api/reservations/:id
- [ ] GET /api/reservations
- [ ] PATCH /api/reservations/:id
- [ ] GET /api/capacity/dashboard
- [ ] GET /api/capacity/zones/:id/stalls
- [ ] GET /api/reservations/:id/timeline

### Notifications
- [ ] GET /api/notifications
- [ ] GET /api/notifications/:id
- [ ] PATCH /api/notifications/:id/read
- [ ] POST /api/notifications/:id/dismiss
- [ ] GET /api/notification-settings
- [ ] PATCH /api/notification-settings
- [ ] GET /api/notification-settings/dnd
- [ ] PATCH /api/notification-settings/dnd
- [ ] GET /api/notification-topics
- [ ] POST /api/notification-topics/:id/subscribe
- [ ] DELETE /api/notification-topics/:id/subscribe

### Campaigns and Tasks
- [ ] GET/POST /api/campaigns
- [ ] GET/PATCH/DELETE /api/campaigns/:id
- [ ] GET/POST /api/campaigns/:id/tasks
- [ ] GET/PATCH /api/tasks/:id
- [ ] POST /api/tasks/:id/complete
- [ ] DELETE /api/tasks/:id

### Devices and Tracking
- [ ] POST /api/device-events
- [ ] POST /api/device-events/replay
- [ ] GET /api/device-events
- [ ] GET /api/device-events/:id
- [ ] GET/POST /api/devices
- [ ] GET/PATCH/DELETE /api/devices/:id
- [ ] POST /api/tracking/location
- [ ] GET /api/tracking/vehicles/:id/positions
- [ ] GET /api/tracking/vehicles/:id/stops

### Exceptions (Dispatch Operator)
- [ ] GET /api/exceptions                        # list open exceptions
- [ ] GET /api/exceptions/:id
- [ ] POST /api/exceptions/:id/acknowledge       # Dispatch Operator acknowledges
- [ ] GET /api/exceptions/history                # acknowledged/closed exceptions

### Segmentation
- [ ] GET/POST /api/tags
- [ ] DELETE /api/tags/:id
- [ ] POST /api/members/:id/tags
- [ ] DELETE /api/members/:id/tags/:tagId
- [ ] GET /api/members/:id/tags
- [ ] GET/POST /api/segments
- [ ] GET/PATCH/DELETE /api/segments/:id
- [ ] POST /api/segments/:id/preview
- [ ] POST /api/segments/:id/run
- [ ] GET /api/segments/:id/runs
- [ ] POST /api/tags/export
- [ ] POST /api/tags/import

### Analytics and Exports
- [ ] GET /api/analytics/occupancy
- [ ] GET /api/analytics/bookings
- [ ] GET /api/analytics/exceptions
- [ ] GET /api/exports
- [ ] POST /api/exports
- [ ] GET /api/exports/:id/download

### Members — Balance and Arrears
- [ ] GET /api/members/:id/balance               # current arrears balance
- [ ] PATCH /api/members/:id/balance             # admin manual adjustment

### Notifications — SMS/Email Export Packages
- [ ] GET /api/notifications/export-packages     # list pending SMS/email packages
- [ ] GET /api/notifications/export-packages/:id/download

### Capacity Snapshots
- [ ] GET /api/capacity/snapshots                # list snapshots per zone

---

## 6. Test Coverage Assessment

### Required Unit Tests
- [ ] auth: password length boundary (11 rejected, 12 accepted)
- [ ] auth: lockout after 5 fails, counter reset after success
- [ ] auth: session inactivity timeout
- [ ] auth: force_password_change blocks routes
- [ ] capacity: atomic hold prevents oversell under concurrency
- [ ] capacity: hold expiry restores stall count
- [ ] capacity: confirm decrements, cancel releases
- [ ] device: deduplication by event key
- [ ] device: out-of-order reordering within 10-min window
- [ ] device: replay does not double-count
- [ ] device: late event flagged, not reordered
- [ ] notification: DND defers job (not drops)
- [ ] notification: frequency cap suppresses 4th reminder
- [ ] notification: exponential backoff retry up to 5 attempts
- [ ] segment: preview returns count before activation
- [ ] segment: nightly job evaluates and updates member set
- [ ] tag: import writes audit log entry
- [ ] export: 403 for wrong role
- [ ] export: 403 if not in required segment
- [ ] reconciliation: compensating release on positive and negative delta
- [ ] tracking: drift smoothing holds suspect position
- [ ] tracking: stop event created at 3-min threshold

### Required Integration/API Tests
- [ ] Full reservation flow: create hold → confirm → stall count decremented
- [ ] Oversell: two concurrent requests for last stall — one wins, one gets 409
- [ ] Cancel flow: cancel → hold released → stall count restored
- [ ] RBAC matrix: each role tested against forbidden endpoints → 403
- [ ] Object-level auth: user A cannot access user B's reservation → 403
- [ ] Admin password reset flow: reset → force_password_change → login blocked → change password → login succeeds
- [ ] Device event replay: same event key replayed → second application blocked
- [ ] Segment export: user not in segment → 403

---

## 7. Business Logic Hard Questions

The inspector must verify the implementation answers each of these correctly:

1. When two requests arrive simultaneously for the last stall in a zone, which one wins? (Atomic hold with row-level lock — verify transaction isolation level is SERIALIZABLE or uses SELECT FOR UPDATE)

2. If a capacity hold expires while the user is on the confirmation screen, what does the user see? (Conflict error on confirm — verify the confirm endpoint re-checks availability after hold expiry)

3. If a device event arrives 11 minutes late, is it applied to capacity counts or only stored? (Stored with late flag — reconciliation handles drift — verify this code path exists)

4. If the same device event key is replayed twice, does the second replay increment capacity counts? (No — verify the replay endpoint checks the idempotency store before applying)

5. If a user's DND window is 22:00–06:00 and a notification is generated at 23:00, when is it delivered? (At 06:00 — verify the job scheduler defers, not drops)

6. If a booking generates 3 reminders in one day and a 4th trigger fires, is the 4th job created in the queue or rejected before queuing? (Rejected before queuing — verify frequency cap check happens before job creation)

7. If an admin resets a user's password, are they forced to change it before accessing any other endpoint? (Yes — verify force_password_change middleware blocks all non-password-change routes)

8. If a Fleet Manager from Org A tries to GET /api/vehicles/:id where the vehicle belongs to Org B, what is the response? (403 — verify object-level authorization in the vehicle handler)

9. If an Auditor tries to DELETE an audit log entry, what happens? (403 at API level AND DB-level: app role has no DELETE on audit_logs — verify both)

10. If a segment export is requested by a user with the correct role but not a member of the required segment, what is the response? (403 — verify segment membership check in export handler)

11. What happens to active reservations when a zone's stall count is reduced below the current number of confirmed reservations? (Decision needed — block the reduction, or allow with warning and flag over-committed reservations)

12. What happens to notification jobs in the queue if the server restarts? (Jobs must survive restart — verify jobs are persisted in PostgreSQL, not in-memory)

13. If a reconciliation run finds a discrepancy of +2 stalls, what compensating action is taken? (Generate 2 compensating release records — verify the reconciliation logic handles both positive and negative deltas)

14. Can an operator replay device events from a specific time range, or only individual events by key? (Decision needed — clarify replay scope in the API design)

15. If a campaign task deadline passes and the task is still incomplete, do reminders continue indefinitely? (Decision needed — the prompt says until marked complete but a maximum cap may be needed)

---

## 8. Task 73 Failure Prevention Checklist

These are the exact issues that caused a partial pass on a similar project. Verify each one explicitly.

### 8.1 Admin password reset — no raw token exposed
- [ ] `POST /api/admin/users/:id/reset-password` response body contains NO token
- [ ] Response only contains a success message and force_password_change confirmation
- [ ] No reset token is generated, stored, or returned anywhere in the codebase
- [ ] Test: call the endpoint, verify response has no `token`, `reset_token`, or similar field

### 8.2 Security constraints enforced, not just configured
- [ ] Device registration check: unregistered device_id on POST /api/device-events → 403 (not just config)
- [ ] RBAC middleware: test that actually sends request with wrong role and gets 403 (not just middleware existence)
- [ ] Session inactivity: test that actually waits past timeout and verifies 401 (use clock mock)
- [ ] Rule: every security constraint has a test that verifies enforcement, not just presence

### 8.3 UI/API edge cases tested
- [ ] Capacity dashboard with zero stalls → API returns 0, UI shows "Full" not blank
- [ ] Activity feed with no events → API returns empty array, UI shows empty state message
- [ ] Notification center with no notifications → 200 with empty items array
- [ ] Segment preview returning zero members → 200 with count: 0 (not 404)
- [ ] Export with no data in range → 200 with empty file + truncated: false

### 8.4 No ReDoS risk from user input
- [ ] Segment filter expressions are structured JSON — no raw regex evaluated
- [ ] Message rule templates use variable substitution — no regex
- [ ] If any regex is added: must have timeout wrapper + max length limit + ReDoS test
- [ ] Test: submit a known ReDoS pattern as input, verify it is rejected or times out safely
