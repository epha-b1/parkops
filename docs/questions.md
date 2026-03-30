# Questions and Clarifications — ParkOps

These are all the vague or unclear points in the prompt that need to be resolved before building starts.

---

## 1. Capacity Hold Timeout — Configurable Per Zone or Global?

**Question:** The prompt sets a default hold timeout of 15 minutes but does not specify whether this can be overridden per zone or rate plan.

**Assumption:** The 15-minute timeout is a system-wide default. Facility Administrators can override it per zone.

**Decision needed:** Confirm whether per-zone override is required or if global default is sufficient.

**Solution:** Add `hold_timeout_minutes` column to zones table defaulting to 15. Reservation creation uses the zone's configured timeout.

---

## 2. Out-of-Order Device Events — What Happens After the 10-Minute Window?

**Question:** The idempotency layer corrects out-of-order arrivals within a 10-minute window. The prompt does not say what happens to events that arrive after that window.

**Assumption:** Events arriving more than 10 minutes late are accepted and stored but not reordered — applied in arrival order and flagged as late. The reconciliation job handles any resulting drift.

**Solution:** Store a `late` flag on device events outside the reorder window. Reconciliation treats late events as a trigger for a compensating check on the affected zone.

---

## 3. DND Schedule — Does It Apply to All Notification Topics?

**Question:** The prompt says users can set DND hours but does not specify whether DND suppresses all topics or only reminders.

**Assumption:** DND applies to all notification topics in the current scope.

**Decision needed:** Should any topic (e.g. critical system alerts) be exempt from DND?

**Solution:** Notification worker checks DND before sending any job. Defer until DND end time.

---

## 4. Segment Export Format — What Is Exported?

**Question:** The prompt says operators can export tag versions for rollback but does not define the format or what data is included.

**Assumption:** A tag version export is a JSON snapshot of all tag assignments for the selected member set, including member ID, tag names, and export timestamp.

**Solution:** Export endpoint produces JSON. Import endpoint reads the same format, validates member IDs, restores tag assignments in a transaction. Both recorded in audit log.

---

## 5. Analytics Export — Is There a Row Limit?

**Question:** The prompt requires CSV/Excel/PDF exports but does not specify size or row limits.

**Assumption:** No hard row limit for CSV. Excel capped at 1,048,576 rows (native limit). PDF capped at 10,000 rows. Exports over limit return a truncated file with a warning.

**Solution:** Export service checks row count before generating. Include `truncated: true` in response header and a note at the top of the file if over limit.

---

## 6. Fleet Manager Scope — Can They See Other Organizations' Vehicles?

**Question:** The prompt says Fleet Managers manage vehicles and drivers but does not specify cross-organization visibility.

**Assumption:** Fleet Managers are scoped to their own organization only.

**Solution:** All vehicle and driver queries filtered by `organization_id` from the authenticated Fleet Manager's account. Cross-organization access returns 403.

---

## 7. Reconciliation Job — Tolerance Band or Zero Tolerance?

**Question:** The prompt says the reconciliation job generates compensating releases when event-derived counts differ from snapshots, but does not define a threshold.

**Assumption:** Any non-zero discrepancy triggers a compensating release. No tolerance band.

**Decision needed:** Should small discrepancies (e.g. ±1 stall) be tolerated or always corrected?

**Solution:** Reconciliation compares event-derived count to snapshot per zone. Any difference generates a compensating record and audit log entry.

---

## 8. Location Drift Smoothing — What Algorithm?

**Question:** The prompt requires smoothing of sudden GPS jumps but does not specify the algorithm.

**Assumption:** Threshold-based filter. If a new position is more than a configurable distance (default 500m) from the last known position within a short time window (default 30 seconds), treat as drift and hold until a second confirming report arrives.

**Solution:** Tracker marks the report as `suspect`. If next report confirms the new position (within 500m of suspect), accept both. Otherwise discard the suspect report.

---

## 9. Rate Plans — How Are They Applied to Reservations?

**Question:** The prompt mentions rate plans as something Facility Administrators manage but does not describe how they are applied — is pricing calculated at reservation time, at confirmation, or displayed only?

**Assumption:** Rate plan is linked to a zone. When a reservation is created, the applicable rate is calculated and stored on the reservation record. Price is shown to the operator before confirmation.

**Decision needed:** Is pricing informational only (for display) or does it drive payment/billing within the system?

---

## 10. Arrears — How Is Overdue Balance Tracked?

**Question:** The prompt mentions "arrears reminders" as a notification topic and uses "overdue balance > $50.00" as a segmentation example, but does not describe how arrears are recorded or updated.

**Assumption:** Members have a balance field that is updated when payments are recorded or when an admin manually adjusts it. Arrears = positive outstanding balance.

**Decision needed:** Is there a payment/billing module, or is balance managed manually by admins? This affects whether arrears notifications are triggered automatically or manually.

---

## 11. Message Rules — What Are They?

**Question:** The prompt lists "message rules" as something Facility Administrators manage but does not define what a message rule is or how it differs from a notification trigger rule.

**Decision:** Message rules are notification trigger rules — they define the condition (e.g. "booking confirmed"), the topic to notify, and the message template. They are the configuration layer that the notification engine evaluates to decide when and what to send. They are separate from notification topics (which users subscribe to) and from notification jobs (which are the delivery instances).

**Solution:** `message_rules` table stores: trigger event type, target topic, message template, active flag. The notification dispatcher evaluates active message rules on each triggering event and creates notification jobs for subscribed users.

---

## 12. Campaigns vs Tasks — What Is the Difference?

**Question:** The prompt describes "a built-in campaign/task area" with "configurable reading and activity tasks." It is unclear whether a campaign is just a container for tasks or has its own lifecycle and targeting.

**Decision:** A campaign is a named container that groups one or more tasks. It has a title, optional description, and optional target audience (all operators, or a specific role). Tasks are the individual actionable items with a description, optional deadline, and reminder interval. Campaigns are visible to all operators by default unless targeted to a specific role. There is no segment-based campaign targeting in the MVP.

**Solution:** `campaigns` table: id, title, description, target_role (nullable), created_by, created_at. `tasks` table: id, campaign_id, description, deadline (nullable), reminder_interval_minutes, completed_at, completed_by.

---

## 13. Incremental UI Updates — SSE or Polling?

**Question:** The prompt says the backend powers "incremental UI updates" but does not specify the mechanism (SSE, WebSocket, polling).

**Assumption:** Start with polling every 10–15 seconds for MVP. SSE can be added later for the activity feed.

**Decision needed:** Is SSE required from day one or is polling acceptable for MVP?

---

## 14. SMS/Email Export Packages — What Format?

**Question:** The prompt says optional SMS/email outputs are produced as "exportable message packages for manual handling." The format is not defined.

**Assumption:** An exportable message package is a JSON or CSV file containing the recipient, message body, channel (SMS/email), and timestamp for each queued notification. An operator downloads this file and sends it manually through an external tool.

**Decision needed:** Confirm the expected format and whether there is a specific structure required for the external tool that will consume these packages.

---

## 15. Audit Log — Which Actions Are Covered?

**Question:** The prompt explicitly mentions "tag changes, exports, and device event replays" as audit log triggers but the list is incomplete for a full system.

**Assumption:** The audit log should also cover: login/logout, failed login attempts, account lockout, role changes, reservation create/confirm/cancel, capacity hold create/release, reconciliation runs, segment activation, campaign/task create/complete, and admin user management actions.

**Decision needed:** Confirm the full list of audited actions before implementation to avoid retrofitting.

---

## 16. Sensitive Fields Encryption — Which Fields Exactly?

**Question:** The prompt says "password hashes, API tokens, and member contact notes" are encrypted at rest. It is unclear whether this means the password hash itself is double-encrypted or just stored securely, and whether there are other fields that should be encrypted.

**Assumption:** Password hashes are stored using a strong hashing algorithm (argon2id) — no additional encryption needed. API tokens and member contact notes are encrypted using AES-256-GCM before storage. The encryption key is loaded from env config, never hardcoded.

**Decision needed:** Confirm whether any other fields (e.g. vehicle plate numbers, driver licence numbers) should also be encrypted at rest.

---

## 17. Role Model — Can a User Have Multiple Roles?

**Question:** The prompt defines four roles (Facility Administrator, Dispatch Operator, Fleet Manager, Auditor) but does not specify whether a single user account can hold multiple roles.

**Assumption:** A user can hold multiple roles. For example, a user could be both a Dispatch Operator and an Auditor. There is no exclusive role constraint unless explicitly required.

**Decision needed:** Confirm whether any role combinations should be blocked (e.g. Auditor + Facility Administrator would be a conflict of interest).

---

## 18. Offline Buffering — On the Device or in the App?

**Question:** The prompt says devices support "offline buffering and retransmission" but does not specify whether this buffering happens on the device firmware side or whether the app server also needs to handle a bulk retransmission endpoint.

**Assumption:** Buffering is on the device side. The app server provides a standard inbound event endpoint. When a device reconnects, it retransmits buffered events to the same endpoint. The idempotency layer handles deduplication.

**Decision needed:** Confirm whether a dedicated bulk retransmission endpoint is needed or if the standard single-event endpoint is sufficient.

---

## 19. Signed Device Time — What Does "Signed" Mean?

**Question:** The prompt says trusted timestamps use "server time plus signed device time when available." It is unclear what "signed" means here — HMAC-signed payload, a certificate, or just a device-provided timestamp that is stored alongside server time.

**Assumption:** "Signed device time" means the device includes its local timestamp in the event payload along with an HMAC signature over the payload using a pre-shared device key. The server stores both timestamps and marks the device time as trusted if the HMAC validates.

**Decision needed:** Confirm the signing mechanism and whether device keys need to be provisioned and managed in the system.

---

## 20. Nightly Segment Schedule — What Time Does It Run?

**Question:** The prompt says segments can run on a nightly schedule but does not specify the time.

**Assumption:** Nightly segment evaluation runs at a configurable time, defaulting to 02:00 local server time.

**Decision needed:** Confirm the default run time and whether it should be configurable per segment or system-wide.

---

## 21. Password Reset — No Email, So How Does It Work?

**Question:** The system is offline/local-only with no external connectivity. There is no email-based forgot password flow. The prompt does not describe how a user recovers access if they forget their password.

**Assumption:** Password reset is admin-initiated only. An admin sets a temporary password for the user via the admin panel. The user is forced to change it on next login. A `force_password_change` flag on the users table controls this.

**Decision needed:** Confirm this is the intended flow. Also confirm whether a user can change their own password while logged in (via a "change password" screen).

---

## 22. Session Management — Can a User Have Multiple Active Sessions?

**Question:** The prompt defines session expiration after 30 minutes of inactivity but does not specify whether a user can be logged in from multiple devices/browsers simultaneously.

**Assumption:** Multiple concurrent sessions are allowed per user. Each session is tracked independently with its own inactivity timer.

**Decision needed:** Should admins be able to force-expire all sessions for a user (e.g. after a security incident)? If yes, a `DELETE /api/admin/users/:id/sessions` endpoint is needed.

---

## 23. Exception Acknowledgement — Who Can Acknowledge and What Happens After?

**Question:** The prompt says Dispatch Operators monitor exceptions but does not define what "acknowledging" an exception means or what state it moves to.

**Assumption:** An exception (e.g. gate stuck open, sensor offline) has a status of `open` or `acknowledged`. A Dispatch Operator can mark it acknowledged with an optional note. Acknowledged exceptions remain visible in history but are removed from the active alerts feed.

**Decision needed:** Can exceptions be re-opened after acknowledgement? Is there an escalation path if an exception is not acknowledged within a time window?

---

## 24. Reservation Update — What Can Be Changed After Creation?

**Question:** The plan includes a `PATCH /api/reservations/:id` endpoint but the prompt does not specify which fields can be updated and under what conditions.

**Assumption:** Only reservations in `hold` or `confirmed` state can be updated. Editable fields are: time window, stall count, notes. Changing the time window or stall count triggers a new availability check and may require releasing and re-creating the capacity hold.

**Decision needed:** Confirm which fields are editable and whether time window changes require a full re-hold or can be done in-place.
