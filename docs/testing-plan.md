# ParkOps — Testing Plan

Based on the repo code gen manual requirements:
- `unit_tests/` — core logic, state transitions, boundary conditions
- `API_tests/` — functional tests covering normal inputs, missing params, permission errors
- `run_tests.sh` — single entrypoint, clear pass/fail summary

Stack: Go + Gin + PostgreSQL. Test framework: `go test` for unit tests, `httptest` or a dedicated HTTP client (e.g. `hurl` or Go's `net/http`) for API tests.

---

## Project Test Structure

```
parkops/
├── unit_tests/
│   ├── auth_test.go
│   ├── capacity_test.go
│   ├── device_test.go
│   ├── notification_test.go
│   ├── segment_test.go
│   ├── tracking_test.go
│   ├── reconciliation_test.go
│   └── export_test.go
├── API_tests/
│   ├── auth_api_test.go
│   ├── reservations_api_test.go
│   ├── capacity_api_test.go
│   ├── devices_api_test.go
│   ├── notifications_api_test.go
│   ├── segments_api_test.go
│   ├── analytics_api_test.go
│   └── rbac_api_test.go
└── run_tests.sh
```

---

## run_tests.sh

```bash
#!/bin/bash
set -e

echo "=== ParkOps Test Suite ==="
echo ""

echo "--- Unit Tests ---"
go test ./unit_tests/... -v -count=1
UNIT_EXIT=$?

echo ""
echo "--- API Tests ---"
go test ./API_tests/... -v -count=1
API_EXIT=$?

echo ""
if [ $UNIT_EXIT -eq 0 ] && [ $API_EXIT -eq 0 ]; then
  echo "=== ALL TESTS PASSED ==="
  exit 0
else
  echo "=== TESTS FAILED ==="
  exit 1
fi
```

---

## Unit Tests

Unit tests cover pure logic with no HTTP or DB. Use mocks/stubs for dependencies.

---

### auth_test.go

| Test | What it verifies |
|---|---|
| `TestPasswordMinLength` | 11-char password rejected, 12-char accepted |
| `TestPasswordRequiresMixedChars` | digits-only or letters-only rejected |
| `TestLockoutAfter5Fails` | 6th attempt returns lockout error |
| `TestLockoutDuration` | lockout expires after 15 minutes |
| `TestLockoutCounterResetOnSuccess` | successful login resets fail counter |
| `TestSessionInactivityTimeout` | session marked expired after 30 min inactivity |
| `TestForcePasswordChangeBlocksRoutes` | user with force_password_change=true blocked from non-password endpoints |
| `TestForcePasswordChangeClearedAfterChange` | flag cleared after successful password change |

---

### capacity_test.go

| Test | What it verifies |
|---|---|
| `TestHoldCreatedOnReservation` | hold record created with correct expiry |
| `TestHoldExpiryRestoresStallCount` | expired hold releases stall back to zone |
| `TestConfirmDecrementsStallCount` | confirm decrements zone stall count |
| `TestCancelReleasesHold` | cancel releases hold and restores stall count |
| `TestOversellRejected` | request for zone with 0 stalls returns conflict error |
| `TestHoldTimeoutIsZoneConfigurable` | zone with custom timeout uses that value |
| `TestConfirmReChecksAvailability` | confirm fails if hold expired before confirmation |
| `TestConcurrentHoldsOversellPrevention` | two concurrent requests for last stall — exactly one succeeds |

---

### device_test.go

| Test | What it verifies |
|---|---|
| `TestEventDeduplicationByKey` | second event with same key is discarded |
| `TestMissingDeviceIDRejected` | event without device_id returns 400 |
| `TestMissingSequenceNumberRejected` | event without sequence_number returns 400 |
| `TestMissingEventKeyRejected` | event without event_key returns 400 |
| `TestOutOfOrderCorrectionWithinWindow` | events within 10-min window reordered by sequence |
| `TestLateEventFlaggedOutsideWindow` | event arriving >10 min late stored with late=true |
| `TestReplayDoesNotDoubleCount` | replayed event key does not re-apply capacity effect |
| `TestReplayWithNewKeyApplied` | replay with new event key is applied normally |

---

### notification_test.go

| Test | What it verifies |
|---|---|
| `TestDNDDefersNotification` | job generated at 23:00 with DND 22:00–06:00 is deferred to 06:00 |
| `TestDNDDoesNotDropJob` | deferred job still exists in queue after DND check |
| `TestFrequencyCapBlocks4thReminder` | 4th reminder for same booking same day is rejected before queuing |
| `TestFrequencyCapResetsDailyPerBooking` | cap resets at midnight per booking |
| `TestExponentialBackoffRetry` | failed job retried with increasing delay |
| `TestMaxRetryAttempts` | job marked failed after 5 attempts |
| `TestInAppDeliveryImmediate` | in-app job delivered without delay |
| `TestSMSExportPackageGenerated` | SMS notification produces exportable package record |

---

### segment_test.go

| Test | What it verifies |
|---|---|
| `TestSegmentPreviewCountBeforeActivation` | preview returns matching member count without activating |
| `TestSegmentFilterByTag` | filter expression matches members with correct tag |
| `TestSegmentFilterByBalance` | filter expression matches members with overdue balance > threshold |
| `TestOnDemandEvaluation` | on-demand run updates member set |
| `TestNightlyScheduleEvaluates` | nightly job evaluates and updates member set |
| `TestTagVersionExportFormat` | export produces JSON with member IDs, tag names, timestamp |
| `TestTagVersionImportRestores` | import restores previous tag assignments |
| `TestTagImportWritesAuditLog` | audit log entry written on import with operator, timestamp, before/after |

---

### tracking_test.go

| Test | What it verifies |
|---|---|
| `TestDriftSmoothingHoldsSuspectPosition` | position jump > 500m within 30s marked suspect |
| `TestDriftConfirmedBySecondReport` | second report within 500m of suspect accepts both |
| `TestDriftDiscardedWithoutConfirmation` | suspect position discarded if next report is far |
| `TestStopDetectionAt3Minutes` | stop event created when vehicle stationary > 3 min |
| `TestStopNotCreatedBefore3Minutes` | no stop event at 2 min 59 sec |
| `TestTrustedTimestampServerTime` | server time always recorded |
| `TestTrustedTimestampSignedDeviceTime` | signed device time stored when HMAC validates |

---

### reconciliation_test.go

| Test | What it verifies |
|---|---|
| `TestReconciliationPositiveDelta` | event-derived count > snapshot → compensating release generated |
| `TestReconciliationNegativeDelta` | event-derived count < snapshot → compensating hold generated |
| `TestReconciliationNoDelta` | no discrepancy → no compensating record |
| `TestReconciliationAuditLogEntry` | audit log entry written for every run |
| `TestReconciliationRunsEvery30Min` | scheduler fires at correct interval |

---

### export_test.go

| Test | What it verifies |
|---|---|
| `TestExportRoleRestriction` | wrong role returns 403 |
| `TestExportSegmentRestriction` | user not in required segment returns 403 |
| `TestCSVExportFormat` | CSV output has correct headers and rows |
| `TestExcelRowLimit` | Excel export truncated at 1,048,576 rows with warning |
| `TestPDFRowLimit` | PDF export truncated at 10,000 rows with warning |
| `TestExportAuditLogEntry` | audit log entry written for every export |

---

## API Tests

API tests run against the real server with a test database. Each test makes real HTTP requests and asserts status codes, response bodies, and side effects.

Setup: spin up the app with `TEST_DB_URL` pointing to a test PostgreSQL instance. Seed with known data before each test group.

---

### auth_api_test.go

| Test | Endpoint | Scenario | Expected |
|---|---|---|---|
| `TestLoginSuccess` | POST /api/auth/login | valid credentials | 200 + session token |
| `TestLoginWrongPassword` | POST /api/auth/login | wrong password | 401 |
| `TestLoginLockout` | POST /api/auth/login | 5 fails then 6th | 429 with retry-after |
| `TestLoginLockedAccount` | POST /api/auth/login | locked account | 429 |
| `TestLogout` | POST /api/auth/logout | valid session | 204 |
| `TestMeUnauthorized` | GET /api/me | no session | 401 |
| `TestChangePassword` | PATCH /api/me/password | valid new password | 200 |
| `TestChangePasswordTooShort` | PATCH /api/me/password | 11-char password | 400 |
| `TestAdminResetPassword` | POST /api/admin/users/:id/reset-password | admin resets | 200 + force_password_change set |
| `TestNonAdminCannotResetPassword` | POST /api/admin/users/:id/reset-password | non-admin | 403 |
| `TestForcePasswordChangeBlocksLogin` | GET /api/me | force_password_change=true | 403 with redirect hint |
| `TestUnlockAccount` | POST /api/admin/users/:id/unlock | admin unlocks | 200 |
| `TestForceExpireSessions` | DELETE /api/admin/users/:id/sessions | admin force-expires | 204 |

---

### reservations_api_test.go

| Test | Endpoint | Scenario | Expected |
|---|---|---|---|
| `TestCreateHold` | POST /api/reservations/hold | valid zone + time window | 201 + hold_id |
| `TestCreateHoldOversell` | POST /api/reservations/hold | zone at capacity | 409 |
| `TestConfirmReservation` | POST /api/reservations/:id/confirm | valid hold | 200 + stall count decremented |
| `TestConfirmExpiredHold` | POST /api/reservations/:id/confirm | hold expired | 409 |
| `TestCancelReservation` | POST /api/reservations/:id/cancel | valid reservation | 200 + stall count restored |
| `TestGetAvailability` | GET /api/availability | zone + time window | 200 + remaining stalls |
| `TestGetReservationTimeline` | GET /api/reservations/:id/timeline | valid id | 200 + event list |
| `TestUpdateReservationBeforeConfirm` | PATCH /api/reservations/:id | in hold state | 200 |
| `TestUpdateReservationAfterConfirm` | PATCH /api/reservations/:id | confirmed state | 409 |
| `TestConcurrentOversell` | POST /api/reservations/hold x2 | last stall | one 201, one 409 |

---

### capacity_api_test.go

| Test | Endpoint | Scenario | Expected |
|---|---|---|---|
| `TestCapacityDashboard` | GET /api/capacity/dashboard | authenticated | 200 + zone stall counts |
| `TestZoneStallsForTimeWindow` | GET /api/capacity/zones/:id/stalls | valid zone + window | 200 + remaining count |
| `TestCapacitySnapshots` | GET /api/capacity/snapshots | authenticated | 200 + snapshot list |

---

### devices_api_test.go

| Test | Endpoint | Scenario | Expected |
|---|---|---|---|
| `TestIngestDeviceEvent` | POST /api/device-events | valid event | 201 |
| `TestIngestMissingDeviceID` | POST /api/device-events | no device_id | 400 |
| `TestIngestDuplicateEventKey` | POST /api/device-events | duplicate key | 200 (idempotent, not re-processed) |
| `TestReplayEvent` | POST /api/device-events/replay | valid replay | 200 |
| `TestReplayDuplicateKey` | POST /api/device-events/replay | already processed key | 200 (no double-count) |
| `TestListDeviceEvents` | GET /api/device-events | with filters | 200 + filtered list |
| `TestRegisterDevice` | POST /api/devices | valid device | 201 |
| `TestLocationReport` | POST /api/tracking/location | valid report | 201 |
| `TestVehiclePositionHistory` | GET /api/tracking/vehicles/:id/positions | valid vehicle | 200 + positions |
| `TestVehicleStops` | GET /api/tracking/vehicles/:id/stops | valid vehicle | 200 + stop events |

---

### notifications_api_test.go

| Test | Endpoint | Scenario | Expected |
|---|---|---|---|
| `TestListNotifications` | GET /api/notifications | authenticated | 200 + list |
| `TestMarkRead` | PATCH /api/notifications/:id/read | valid id | 200 |
| `TestDismiss` | POST /api/notifications/:id/dismiss | valid id | 200 |
| `TestGetDNDSettings` | GET /api/notification-settings/dnd | authenticated | 200 + dnd config |
| `TestUpdateDNDSettings` | PATCH /api/notification-settings/dnd | valid times | 200 |
| `TestSubscribeToTopic` | POST /api/notification-topics/:id/subscribe | valid topic | 200 |
| `TestUnsubscribeFromTopic` | DELETE /api/notification-topics/:id/subscribe | subscribed | 200 |
| `TestExportPackagesList` | GET /api/notifications/export-packages | authenticated | 200 + list |
| `TestExportPackageDownload` | GET /api/notifications/export-packages/:id/download | valid id | 200 + file |

---

### segments_api_test.go

| Test | Endpoint | Scenario | Expected |
|---|---|---|---|
| `TestCreateSegment` | POST /api/segments | valid filter | 201 |
| `TestSegmentPreview` | POST /api/segments/:id/preview | valid segment | 200 + count |
| `TestRunSegmentOnDemand` | POST /api/segments/:id/run | valid segment | 200 + member set |
| `TestSegmentRunHistory` | GET /api/segments/:id/runs | valid segment | 200 + run list |
| `TestAddTagToMember` | POST /api/members/:id/tags | valid tag | 200 |
| `TestRemoveTagFromMember` | DELETE /api/members/:id/tags/:tagId | valid tag | 200 |
| `TestExportTagVersion` | POST /api/tags/export | valid member set | 200 + JSON snapshot |
| `TestImportTagVersion` | POST /api/tags/import | valid snapshot | 200 + audit log entry |

---

### analytics_api_test.go

| Test | Endpoint | Scenario | Expected |
|---|---|---|---|
| `TestOccupancyTrend` | GET /api/analytics/occupancy | authenticated | 200 + trend data |
| `TestBookingDistribution` | GET /api/analytics/bookings | with pivot params | 200 + pivot data |
| `TestExportCSV` | POST /api/exports | format=csv | 201 + export_id |
| `TestExportDownload` | GET /api/exports/:id/download | valid export | 200 + file |
| `TestExportWrongRole` | POST /api/exports | wrong role | 403 |
| `TestExportSegmentRestriction` | POST /api/exports | not in segment | 403 |
| `TestMemberBalance` | GET /api/members/:id/balance | valid member | 200 + balance |
| `TestAdjustMemberBalance` | PATCH /api/members/:id/balance | admin | 200 + audit log |
| `TestAdjustMemberBalanceNonAdmin` | PATCH /api/members/:id/balance | non-admin | 403 |

---

### rbac_api_test.go

This file tests the full RBAC matrix — every role against every forbidden endpoint.

| Test | What it verifies |
|---|---|
| `TestBuyerCannotCreateListing` | Buyer → POST /api/lots → 403 |
| `TestDispatchOperatorCannotManageLots` | Dispatch Operator → POST /api/lots → 403 |
| `TestFleetManagerCannotSeeOtherOrgVehicles` | Fleet Manager → GET /api/vehicles/:id (other org) → 403 |
| `TestAuditorCannotModifyReservation` | Auditor → POST /api/reservations → 403 |
| `TestNonAdminCannotAccessAdminUsers` | non-admin → GET /api/admin/users → 403 |
| `TestNonAuditorCannotDeleteAuditLog` | any role → DELETE /api/admin/audit-logs/:id → 403 |
| `TestObjectLevelAuthReservation` | User A → GET /api/reservations/:id (User B's) → 403 |
| `TestObjectLevelAuthDevice` | non-owner → PATCH /api/devices/:id → 403 |
| `TestExceptionAcknowledgementRoleCheck` | non-Dispatch Operator → POST /api/exceptions/:id/acknowledge → 403 |
| `TestAuditLogReadOnlyForAuditor` | Auditor → GET /api/admin/audit-logs → 200 |
| `TestAuditLogForbiddenForOthers` | Fleet Manager → GET /api/admin/audit-logs → 403 |

---

## Test Database Setup

Each API test group uses a shared test DB helper:

```go
// API_tests/testdb_test.go
func setupTestDB(t *testing.T) *sql.DB {
    db := connectToTestDB()
    runMigrations(db)
    seedTestData(db)
    t.Cleanup(func() { truncateAllTables(db) })
    return db
}
```

Seed data includes:
- One user per role (facility_admin, dispatch_operator, fleet_manager, auditor)
- Two organizations (Org A, Org B) for cross-org isolation tests
- One zone per org with known stall counts
- One device per org

---

## Coverage Summary

| Area | Unit | API | Priority |
|---|---|---|---|
| Auth / lockout / session | auth_test.go | auth_api_test.go | Blocking |
| Capacity hold / oversell | capacity_test.go | reservations_api_test.go | Blocking |
| Device idempotency / replay | device_test.go | devices_api_test.go | Blocking |
| RBAC matrix | — | rbac_api_test.go | Blocking |
| Object-level auth | — | rbac_api_test.go | Blocking |
| Notification DND / freq cap | notification_test.go | notifications_api_test.go | High |
| Segmentation / tag import | segment_test.go | segments_api_test.go | High |
| Reconciliation | reconciliation_test.go | — | High |
| Tracking / drift / stops | tracking_test.go | devices_api_test.go | Medium |
| Export restrictions | export_test.go | analytics_api_test.go | High |
| Audit log integrity | — | rbac_api_test.go | High |
| Concurrent oversell | capacity_test.go | reservations_api_test.go | Blocking |
