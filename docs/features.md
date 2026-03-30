# ParkOps — Feature Overview

Offline-first parking operations platform built with Go (Gin) + Templ + PostgreSQL.

---

## Reservations & Capacity

What it does: Operators book stalls in a zone for a time window.

What needs to be built:
- Create reservation endpoint that atomically holds capacity (15-min default timeout)
- Confirm reservation endpoint that decrements stall count and releases hold
- Cancel reservation endpoint that releases hold and restores stall count
- Auto-expiry job that releases timed-out holds
- Conflict check that rejects bookings when no stalls are available

---

## Operator Console

What it does: Web UI for monitoring and managing parking operations in real time.

What needs to be built:
- Activity feed page that polls or streams new device events and reservation changes
- Reservation calendar view showing bookings per zone per day/hour
- Capacity dashboard showing remaining stalls per zone for a selected time window
- Conflict warning component shown before a booking is confirmed

---

## Roles & Access Control

What it does: Each user type sees and can do only what their role allows.

What needs to be built:
- Role definitions: Facility Administrator, Dispatch Operator, Fleet Manager, Auditor
- Middleware that checks role on every API endpoint
- 403 response + audit log entry on denied access
- Role assignment UI for administrators

---

## Device Integration

What it does: Cameras, gates, and geomagnetic sensors send events over the local network.

What needs to be built:
- Inbound event endpoint that validates device ID, sequence number, and unique event key
- Idempotency layer that deduplicates events by event key
- Out-of-order correction that buffers and reorders events within a 10-minute window
- Controlled replay support that replays events without double-counting
- Offline buffer and retransmission support on the device side

---

## Inventory Reconciliation

What it does: Keeps stall counts accurate even when device events arrive late.

What needs to be built:
- Scheduled job that runs every 30 minutes
- Comparison logic between event-derived counts and authoritative capacity snapshots
- Compensating release records generated when a discrepancy is found
- Audit log entry for each reconciliation run and its outcomes

---

## Real-time Tracking

What it does: Tracks vehicle and mobile positions, smooths bad data, detects stops.

What needs to be built:
- Location report ingestion endpoint
- Drift smoothing algorithm that filters sudden positional jumps
- Stop detection that flags vehicles stationary for more than 3 minutes
- Trusted timestamp recording using server time plus optional signed device time
- Storage of location reports and stop events in PostgreSQL

---

## Notifications

What it does: Alerts users about booking events, expiry, and arrears with delivery controls.

What needs to be built:
- Trigger rule engine that generates notification jobs on booking success, changes, expiry, and arrears
- Internal notification queue with worker that processes jobs
- Exponential backoff retry logic with max 5 attempts
- In-app delivery that shows notifications immediately in the UI
- SMS/email exportable message package generator (no external sending)
- DND schedule configuration per user (start time, end time)
- Frequency cap enforcement (max 3 reminders per booking per day)
- Suppression log when a notification is blocked by DND or frequency cap

---

## Campaigns & Tasks

What it does: Operators create task lists with deadlines that remind staff until done.

What needs to be built:
- Campaign create/edit UI and API
- Task create/edit with description and optional deadline
- Reminder job generator that fires at configured intervals until task is marked complete
- Mark complete endpoint that stops reminders
- Campaign and task persistence in PostgreSQL

---

## Member Tagging & Segmentation

What it does: Tag members and build filter-based segments for targeted operations.

What needs to be built:
- Tag apply/remove endpoints for member records
- Segment definition builder with filter expressions (tags + attributes like overdue balance)
- Segment preview that returns matching member count before activation
- On-demand segment evaluation endpoint
- Nightly scheduled segment evaluation job
- Tag version export (snapshot of current tag state)
- Tag version import (restore a previous snapshot) with audit log entry
- Persistence of tags, segment definitions, and tag version history in PostgreSQL

---

## Analytics & Reporting

What it does: Pivot operational data and export reports in multiple formats.

What needs to be built:
- Aggregation queries for pivots by time, region, category, entity, and risk level
- Trend chart and distribution chart data endpoints
- Export endpoints producing CSV, Excel, and PDF
- Role-based access check before generating any export
- Segment membership check for segment-restricted datasets
- Audit log entry for every export (user, timestamp, format, scope)

---

## Security

What it does: Local-only auth with lockout, encryption, and a tamper-evident audit log.

What needs to be built:
- Username/password login with minimum 12-character password enforcement
- Account lockout after 5 failed attempts for 15 minutes
- Session inactivity timeout at 30 minutes
- Encrypted storage for password hashes, API tokens, and member contact notes
- Append-only audit log table with no update/delete permissions for app roles
- Audit log entries for tag changes, exports, device replays, denied access, lockouts, and session events
