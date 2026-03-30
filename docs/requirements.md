# Requirements Document

## Introduction

ParkOps Command & Reservation is an offline-first parking operations platform built with Go (Gin), Templ, and PostgreSQL. It enables parking operators to manage reservations, capacity, device events, notifications, member segmentation, and analytics entirely on a local network without external connectivity. Four distinct roles — Facility Administrator, Dispatch Operator, Fleet Manager, and Auditor — interact through a Templ-rendered web UI backed by a decoupled REST API layer.

## Glossary

- **System**: The ParkOps Command & Reservation platform as a whole.
- **API_Server**: The Gin-based HTTP server that exposes REST-style endpoints.
- **UI**: The Templ-rendered web interface served by the API_Server.
- **Facility_Administrator**: A user role responsible for managing lots, zones, rate plans, and message rules.
- **Dispatch_Operator**: A user role responsible for monitoring live movements and exceptions.
- **Fleet_Manager**: A user role (business member) responsible for managing vehicles and drivers.
- **Auditor**: A read-only user role that reviews immutable histories and exports.
- **Reservation**: A time-bounded booking of one or more stalls in a zone.
- **Capacity_Hold**: A temporary atomic reservation of stall capacity created at booking time, with a default timeout of 15 minutes.
- **Zone**: A named subdivision of a parking lot containing a defined number of stalls.
- **Stall**: An individual parking space within a Zone.
- **Rate_Plan**: A pricing schedule associated with a Zone or Lot.
- **Consistency_Engine**: The subsystem that enforces oversell prevention through atomic holds, confirmations, and releases.
- **Reconciliation_Job**: A scheduled background process that compares event-derived capacity counts against authoritative snapshots.
- **Device**: A parking infrastructure device — camera, gate, or geomagnetic sensor — that connects over the local network.
- **Device_Event**: An inbound message from a Device carrying a device identifier, monotonic sequence number, and unique event key.
- **Idempotency_Layer**: The subsystem that deduplicates Device_Events and corrects out-of-order arrivals.
- **Tracker**: The subsystem that accepts vehicle and mobile location reports, smooths drift, and detects stops.
- **Notification_Queue**: The internal queue that holds notification jobs pending delivery.
- **Notification_Job**: A single notification delivery attempt associated with a trigger rule and a recipient.
- **DND_Schedule**: A per-user Do Not Disturb time window during which notifications are suppressed.
- **Frequency_Cap**: A per-booking limit on the number of reminder notifications delivered per day.
- **Campaign**: A configurable set of tasks that generate reminders until marked complete.
- **Task**: A single actionable item within a Campaign with an optional deadline.
- **Tag**: A label applied to a member for segmentation purposes.
- **Segment**: A named filter expression over member tags and attributes used for targeted operations.
- **Segment_Preview**: A count of members matching a Segment definition before activation.
- **Analytics_Engine**: The subsystem that aggregates operational data and produces pivot reports and charts.
- **Audit_Log**: An append-only, tamper-evident record of security-sensitive and data-changing operations.
- **Session**: An authenticated user session tracked server-side with an inactivity timeout.

---

## Requirements

### Requirement 1: User Roles and Access Control

**User Story:** As a Facility_Administrator, I want role-based access enforced on every operation, so that each user can only perform actions appropriate to their role.

#### Acceptance Criteria

1. THE API_Server SHALL enforce role-based access checks on every API endpoint before processing the request.
2. WHEN a Facility_Administrator is authenticated, THE API_Server SHALL permit management of lots, zones, rate plans, and message rules.
3. WHEN a Dispatch_Operator is authenticated, THE API_Server SHALL permit read access to live movements and exceptions and permit acknowledgement of exceptions.
4. WHEN a Fleet_Manager is authenticated, THE API_Server SHALL permit management of vehicles and drivers associated with the Fleet_Manager's organization.
5. WHEN an Auditor is authenticated, THE API_Server SHALL permit read-only access to reservation history, device event history, audit logs, and exports.
6. IF a request is received for an endpoint that the authenticated user's role does not permit, THEN THE API_Server SHALL return an HTTP 403 response and record the attempt in the Audit_Log.

---

### Requirement 2: Authentication and Session Security

**User Story:** As a Facility_Administrator, I want secure local authentication with account lockout, so that unauthorized access to the platform is prevented.

#### Acceptance Criteria

1. THE System SHALL authenticate users using username and password only, without requiring external connectivity.
2. WHEN a user attempts to register or change a password, THE System SHALL reject passwords shorter than 12 characters.
3. WHEN a user provides incorrect credentials 5 consecutive times, THE System SHALL lock the account for 15 minutes and reject further login attempts during that period.
4. WHILE a Session is active, THE System SHALL expire the Session after 30 minutes of inactivity and require re-authentication.
5. THE System SHALL store password hashes, API tokens, and member contact notes encrypted at rest using an authenticated encryption scheme.
6. IF a locked account receives a login attempt before the 15-minute lockout period expires, THEN THE API_Server SHALL return an HTTP 429 response indicating the remaining lockout duration.

---

### Requirement 3: Operator Console Web UI

**User Story:** As a Dispatch_Operator, I want a real-time operator console, so that I can monitor parking activity and respond to exceptions without delay.

#### Acceptance Criteria

1. THE UI SHALL render an operator console page containing a real-time activity feed, a reservation calendar, and a capacity dashboard.
2. WHEN a new Device_Event or reservation state change occurs, THE UI SHALL update the activity feed within 5 seconds without requiring a full page reload.
3. THE UI SHALL display remaining stalls per Zone for the selected time window on the capacity dashboard.
4. WHEN a Dispatch_Operator selects a time window on the capacity dashboard, THE UI SHALL display remaining stall counts for each Zone within that window.
5. WHEN a Facility_Administrator or Dispatch_Operator initiates a reservation, THE UI SHALL display a conflict warning before confirmation if the requested Zone has insufficient remaining capacity for the requested time window.
6. THE System SHALL maintain an exceptions list for device-generated anomalies (e.g. gate stuck open, sensor offline) with status `open` or `acknowledged`.
7. WHEN a Dispatch_Operator acknowledges an exception, THE System SHALL transition its status to `acknowledged`, record the operator identity and an optional note, and remove it from the active alerts feed.
8. Acknowledged exceptions SHALL remain visible in exception history and SHALL NOT be deleted.

---

### Requirement 4: Reservation Management

**User Story:** As a Facility_Administrator, I want to create, confirm, and cancel reservations, so that stall allocation is tracked accurately.

#### Acceptance Criteria

1. WHEN a reservation creation request is received, THE Consistency_Engine SHALL atomically create a Capacity_Hold for the requested stalls in the target Zone.
2. WHEN a Capacity_Hold is created, THE Consistency_Engine SHALL set the hold expiration to 15 minutes from creation time unless a different timeout is specified.
3. WHEN a reservation is confirmed, THE Consistency_Engine SHALL decrement the available stall count for the Zone and release the associated Capacity_Hold.
4. WHEN a reservation is cancelled, THE Consistency_Engine SHALL release the associated Capacity_Hold and restore the stall count to the Zone.
5. WHEN a Capacity_Hold reaches its expiration time without confirmation, THE Consistency_Engine SHALL release the hold and restore the stall count to the Zone.
6. IF a reservation creation request is received for a Zone with no available stalls for the requested time window, THEN THE Consistency_Engine SHALL reject the request and return a conflict error without creating a Capacity_Hold.
7. THE System SHALL persist reservation records, Capacity_Hold records, and capacity snapshots in PostgreSQL.

---

### Requirement 5: Inventory and Availability Consistency

**User Story:** As a Facility_Administrator, I want a reconciliation job to correct capacity drift, so that stall counts remain accurate even when device events arrive late.

#### Acceptance Criteria

1. THE Reconciliation_Job SHALL execute every 30 minutes.
2. WHEN the Reconciliation_Job executes, THE Reconciliation_Job SHALL compare event-derived stall counts against the authoritative capacity snapshots stored in PostgreSQL for each Zone.
3. WHEN the Reconciliation_Job detects a discrepancy between event-derived counts and authoritative snapshots, THE Reconciliation_Job SHALL generate compensating release records to correct the stall count.
4. THE Reconciliation_Job SHALL record each reconciliation run and its outcomes in the Audit_Log.
5. THE System SHALL persist capacity snapshots in PostgreSQL as the authoritative source of truth for stall availability.

---

### Requirement 6: Parking Device Integration

**User Story:** As a Dispatch_Operator, I want parking devices to send events reliably over the local network, so that gate, camera, and sensor data is captured without data loss.

#### Acceptance Criteria

1. THE System SHALL accept Device_Events from cameras, gates, and geomagnetic sensors over the local network.
2. THE System SHALL support offline buffering and retransmission for devices that temporarily lose connectivity to the API_Server.
3. WHEN a Device_Event is received, THE Idempotency_Layer SHALL verify that the event includes a device identifier, a monotonic sequence number, and a unique event key.
4. IF a Device_Event is received without a device identifier, monotonic sequence number, or unique event key, THEN THE API_Server SHALL reject the event with an HTTP 400 response.
5. WHEN a Device_Event is received with a unique event key that matches a previously processed event, THE Idempotency_Layer SHALL discard the duplicate without reprocessing.
6. WHEN Device_Events arrive out of order within a 10-minute window, THE Idempotency_Layer SHALL reorder and process them in sequence number order.
7. WHEN a controlled replay of Device_Events is requested, THE Idempotency_Layer SHALL process replayed events without double-counting previously applied effects.

---

### Requirement 7: Real-Time Vehicle and Location Tracking

**User Story:** As a Dispatch_Operator, I want accurate real-time location data for vehicles, so that I can monitor movements and detect prolonged stops.

#### Acceptance Criteria

1. THE Tracker SHALL accept vehicle and mobile location reports submitted over the local network.
2. WHEN a location report is received that represents a sudden positional jump inconsistent with normal vehicle movement, THE Tracker SHALL apply smoothing to correct the drift before storing the position.
3. WHEN a vehicle remains stationary for more than 3 minutes, THE Tracker SHALL record a stop event associated with that vehicle.
4. WHEN a location report is received, THE Tracker SHALL record a trusted timestamp composed of server-received time and, where available, a signed device-provided timestamp.
5. THE Tracker SHALL persist location reports and stop events in PostgreSQL.

---

### Requirement 8: Notification Subscriptions and Delivery

**User Story:** As a Fleet_Manager, I want to subscribe to notification topics and control delivery preferences, so that I receive relevant alerts without being overwhelmed.

#### Acceptance Criteria

1. THE System SHALL allow users to subscribe to notification topics including booking success, booking changes, approaching expiration, and arrears reminders.
2. WHEN a trigger rule condition is met, THE System SHALL generate a Notification_Job and enqueue it in the Notification_Queue.
3. WHEN a Notification_Job is dequeued for in-app delivery, THE System SHALL deliver the notification immediately within the UI.
4. WHEN a Notification_Job fails to deliver, THE System SHALL retry delivery using exponential backoff with a maximum of 5 attempts before marking the job as failed.
5. WHERE SMS or email output is configured, THE System SHALL produce the notification as an exportable message package suitable for manual handling without requiring external connectivity.
6. WHEN a Notification_Job is ready for delivery and the recipient has a DND_Schedule that covers the current time, THE System SHALL defer delivery until the DND_Schedule window ends.
7. WHEN a Notification_Job would exceed the Frequency_Cap of 3 reminder notifications per booking per day for the recipient, THE System SHALL suppress the notification and record the suppression.
8. THE System SHALL allow users to configure DND_Schedule hours specifying a start time and end time (for example, 22:00–06:00).

---

### Requirement 9: Campaigns and Tasks

**User Story:** As a Facility_Administrator, I want to create campaigns with configurable tasks and deadlines, so that operators are reminded to complete operational activities.

#### Acceptance Criteria

1. THE System SHALL allow Facility_Administrators to create Campaigns containing one or more Tasks.
2. WHEN a Task is created, THE System SHALL allow the Facility_Administrator to specify a task description and an optional deadline time.
3. WHEN a Task has a deadline and the Task has not been marked complete, THE System SHALL generate reminder Notification_Jobs at configured intervals until the Task is marked complete.
4. WHEN a Task is marked complete by an authorized user, THE System SHALL stop generating reminder Notification_Jobs for that Task.
5. THE System SHALL persist Campaign and Task records, including completion status and timestamps, in PostgreSQL.

---

### Requirement 10: Member Tagging and Segmentation

**User Story:** As a Facility_Administrator, I want to tag members and define segments with filter conditions, so that I can target operational actions to specific member groups.

#### Acceptance Criteria

1. THE System SHALL allow Facility_Administrators to apply one or more Tags to member records.
2. THE System SHALL allow Facility_Administrators to define Segments using filter expressions over Tags and member attributes (for example, "Downtown monthly permits AND overdue balance > $50.00").
3. WHEN a Segment definition is saved, THE System SHALL compute and display a Segment_Preview showing the count of members matching the definition before the Segment is activated.
4. WHEN a Segment is activated on-demand, THE System SHALL evaluate the Segment filter expression against current member data and produce the matching member set.
5. WHEN a Segment is configured for nightly scheduling, THE System SHALL evaluate the Segment filter expression once per night and update the matching member set.
6. THE System SHALL allow Facility_Administrators to export the current Tag version for a member set and import a previously exported Tag version to restore it.
7. WHEN a Tag version is imported, THE System SHALL record the import operation in the Audit_Log including the operator identity, timestamp, and the previous and restored Tag states.
8. THE System SHALL persist Tag assignments, Segment definitions, and Tag version history in PostgreSQL.

---

### Requirement 11: Analytics and Reporting

**User Story:** As an Auditor, I want to pivot operational data across multiple dimensions and export reports, so that I can review trends and share findings with authorized stakeholders.

#### Acceptance Criteria

1. THE Analytics_Engine SHALL provide pivot views over operational data by time period, region, category, entity, and risk level.
2. THE Analytics_Engine SHALL render trend charts and distribution charts for the selected pivot dimensions.
3. WHEN an authorized user requests a data export, THE Analytics_Engine SHALL produce the export in CSV, Excel, or PDF format as selected.
4. WHEN a data export is requested, THE API_Server SHALL verify that the requesting user's role permits access to the requested data before generating the export.
5. WHEN a data export is requested for a Segment-restricted dataset, THE API_Server SHALL verify that the requesting user is a member of the required Segment before generating the export.
6. THE System SHALL record every export operation in the Audit_Log including the requesting user, timestamp, format, and data scope.

---

### Requirement 12: Audit Log Integrity

**User Story:** As an Auditor, I want an immutable, tamper-evident audit log, so that I can verify the integrity of all security-sensitive operations.

#### Acceptance Criteria

1. THE Audit_Log SHALL record all tag changes, data exports, device event replays, role permission checks that result in denial, account lockout events, and Session creation and expiration events.
2. THE System SHALL append Audit_Log entries without modifying or deleting existing entries.
3. WHEN an Audit_Log entry is written, THE System SHALL include the actor identity, operation type, affected resource identifier, and a server-assigned timestamp.
4. THE System SHALL provide Auditors with a read-only interface to query and filter Audit_Log entries.
5. THE System SHALL persist Audit_Log entries in PostgreSQL in an append-only table with no update or delete permissions granted to application roles.

---

### Requirement 14: Member Arrears Balance

**User Story:** As a Facility_Administrator, I want to track member overdue balances, so that I can use arrears as a segmentation condition and trigger arrears reminder notifications.

#### Acceptance Criteria

1. THE System SHALL maintain an `arrears_balance_cents` field on each member record representing the outstanding overdue amount.
2. THE System SHALL allow Facility_Administrators to manually adjust a member's arrears balance with a reason note.
3. WHEN a member's arrears balance is greater than zero, THE System SHALL be capable of generating an arrears reminder Notification_Job for that member.
4. THE System SHALL expose the arrears balance via the member detail API endpoint.
5. THE System SHALL record every balance adjustment in the Audit_Log including the operator identity, previous balance, new balance, and reason.

**User Story:** As a Facility_Administrator, I want the platform to operate entirely on the local network without external connectivity, so that parking operations continue uninterrupted regardless of internet availability.

#### Acceptance Criteria

1. THE System SHALL operate all reservation, capacity, device integration, notification, segmentation, and analytics functions without requiring any external network connectivity.
2. THE System SHALL use PostgreSQL as the local system of record for reservations, capacity snapshots, device events, notification jobs, tags, segment definitions, and audit trails.
3. THE API_Server SHALL serve both full Templ-rendered pages and incremental UI updates from the local network.
4. WHEN the API_Server starts, THE System SHALL verify that the PostgreSQL database is reachable and apply any pending schema migrations before accepting requests.
