package API_tests

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type reservationFixture struct {
	facilityID string
	lotID      string
	zoneID     string
	memberID   string
	vehicleID  string
}

func createReservationFixture(t *testing.T, env *apiTestEnv, admin *http.Cookie, totalStalls int, holdTimeout int) reservationFixture {
	t.Helper()

	fac := apiRequest(t, env.r, http.MethodPost, "/api/facilities", map[string]any{"name": "F-Res", "address": "Main"}, admin)
	logStep(t, "POST", "/api/facilities", fac.Code, fac.Body.String())
	if fac.Code != http.StatusCreated {
		t.Fatalf("create facility: %d %s", fac.Code, fac.Body.String())
	}
	facilityID := extractID(t, fac.Body.String())

	lot := apiRequest(t, env.r, http.MethodPost, "/api/lots", map[string]any{"facility_id": facilityID, "name": "L-Res"}, admin)
	logStep(t, "POST", "/api/lots", lot.Code, lot.Body.String())
	if lot.Code != http.StatusCreated {
		t.Fatalf("create lot: %d %s", lot.Code, lot.Body.String())
	}
	lotID := extractID(t, lot.Body.String())

	zone := apiRequest(t, env.r, http.MethodPost, "/api/zones", map[string]any{
		"lot_id":               lotID,
		"name":                 "Z-Res",
		"total_stalls":         totalStalls,
		"hold_timeout_minutes": holdTimeout,
	}, admin)
	logStep(t, "POST", "/api/zones", zone.Code, zone.Body.String())
	if zone.Code != http.StatusCreated {
		t.Fatalf("create zone: %d %s", zone.Code, zone.Body.String())
	}
	zoneID := extractID(t, zone.Body.String())

	member := apiRequest(t, env.r, http.MethodPost, "/api/members", map[string]any{"display_name": "Member Res", "contact_notes": "n"}, admin)
	logStep(t, "POST", "/api/members", member.Code, member.Body.String())
	if member.Code != http.StatusCreated {
		t.Fatalf("create member: %d %s", member.Code, member.Body.String())
	}
	memberID := extractID(t, member.Body.String())

	vehicle := apiRequest(t, env.r, http.MethodPost, "/api/vehicles", map[string]any{"plate_number": "RES-123", "make": "F", "model": "M"}, admin)
	logStep(t, "POST", "/api/vehicles", vehicle.Code, vehicle.Body.String())
	if vehicle.Code != http.StatusCreated {
		t.Fatalf("create vehicle: %d %s", vehicle.Code, vehicle.Body.String())
	}
	vehicleID := extractID(t, vehicle.Body.String())

	return reservationFixture{facilityID: facilityID, lotID: lotID, zoneID: zoneID, memberID: memberID, vehicleID: vehicleID}
}

func availabilityPath(zoneID string, start, end time.Time) string {
	return "/api/availability?zone_id=" + zoneID +
		"&time_window_start=" + start.UTC().Format(time.RFC3339) +
		"&time_window_end=" + end.UTC().Format(time.RFC3339)
}

func TestReservationFlowAndCapacityEndpoints(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fx := createReservationFixture(t, env, admin, 2, 15)

	start := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	end := start.Add(2 * time.Hour)

	avail0 := apiRequest(t, env.r, http.MethodGet, availabilityPath(fx.zoneID, start, end), nil, admin)
	logStep(t, "GET", "/api/availability", avail0.Code, avail0.Body.String())
	if avail0.Code != http.StatusOK || !strings.Contains(avail0.Body.String(), `"available_stalls":2`) {
		t.Fatalf("initial availability invalid: %d %s", avail0.Code, avail0.Body.String())
	}

	hold := apiRequest(t, env.r, http.MethodPost, "/api/reservations/hold", map[string]any{
		"zone_id":           fx.zoneID,
		"member_id":         fx.memberID,
		"vehicle_id":        fx.vehicleID,
		"time_window_start": start.Format(time.RFC3339),
		"time_window_end":   end.Format(time.RFC3339),
		"stall_count":       1,
	}, admin)
	logStep(t, "POST", "/api/reservations/hold", hold.Code, hold.Body.String())
	if hold.Code != http.StatusCreated {
		t.Fatalf("create hold failed: %d %s", hold.Code, hold.Body.String())
	}
	reservationID := extractID(t, hold.Body.String())

	avail1 := apiRequest(t, env.r, http.MethodGet, availabilityPath(fx.zoneID, start, end), nil, admin)
	logStep(t, "GET", "/api/availability", avail1.Code, avail1.Body.String())
	if avail1.Code != http.StatusOK || !strings.Contains(avail1.Body.String(), `"available_stalls":1`) {
		t.Fatalf("availability after hold invalid: %d %s", avail1.Code, avail1.Body.String())
	}

	confirm := apiRequest(t, env.r, http.MethodPost, "/api/reservations/"+reservationID+"/confirm", nil, admin)
	logStep(t, "POST", "/api/reservations/:id/confirm", confirm.Code, confirm.Body.String())
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm failed: %d %s", confirm.Code, confirm.Body.String())
	}

	timeline := apiRequest(t, env.r, http.MethodGet, "/api/reservations/"+reservationID+"/timeline", nil, admin)
	logStep(t, "GET", "/api/reservations/:id/timeline", timeline.Code, timeline.Body.String())
	if timeline.Code != http.StatusOK || !strings.Contains(timeline.Body.String(), "hold_created") || !strings.Contains(timeline.Body.String(), "confirmed") {
		t.Fatalf("timeline invalid: %d %s", timeline.Code, timeline.Body.String())
	}

	cancel := apiRequest(t, env.r, http.MethodPost, "/api/reservations/"+reservationID+"/cancel", nil, admin)
	logStep(t, "POST", "/api/reservations/:id/cancel", cancel.Code, cancel.Body.String())
	if cancel.Code != http.StatusOK {
		t.Fatalf("cancel failed: %d %s", cancel.Code, cancel.Body.String())
	}

	avail2 := apiRequest(t, env.r, http.MethodGet, availabilityPath(fx.zoneID, start, end), nil, admin)
	logStep(t, "GET", "/api/availability", avail2.Code, avail2.Body.String())
	if avail2.Code != http.StatusOK || !strings.Contains(avail2.Body.String(), `"available_stalls":2`) {
		t.Fatalf("availability after cancel invalid: %d %s", avail2.Code, avail2.Body.String())
	}

	dash := apiRequest(t, env.r, http.MethodGet, "/api/capacity/dashboard", nil, admin)
	logStep(t, "GET", "/api/capacity/dashboard", dash.Code, dash.Body.String())
	if dash.Code != http.StatusOK || !strings.Contains(dash.Body.String(), fx.zoneID) {
		t.Fatalf("dashboard invalid: %d %s", dash.Code, dash.Body.String())
	}

	zoneStalls := apiRequest(t, env.r, http.MethodGet, "/api/capacity/zones/"+fx.zoneID+"/stalls?time_window_start="+start.Format(time.RFC3339)+"&time_window_end="+end.Format(time.RFC3339), nil, admin)
	logStep(t, "GET", "/api/capacity/zones/:id/stalls", zoneStalls.Code, zoneStalls.Body.String())
	if zoneStalls.Code != http.StatusOK {
		t.Fatalf("zone stalls failed: %d %s", zoneStalls.Code, zoneStalls.Body.String())
	}

	snap := apiRequest(t, env.r, http.MethodGet, "/api/capacity/snapshots?limit=10", nil, admin)
	logStep(t, "GET", "/api/capacity/snapshots", snap.Code, snap.Body.String())
	if snap.Code != http.StatusOK || !strings.Contains(snap.Body.String(), fx.zoneID) {
		t.Fatalf("snapshots invalid: %d %s", snap.Code, snap.Body.String())
	}
}

func TestReservationConfirmExpiredHold(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fx := createReservationFixture(t, env, admin, 2, 15)

	start := time.Now().UTC().Add(3 * time.Hour).Truncate(time.Second)
	end := start.Add(time.Hour)

	hold := apiRequest(t, env.r, http.MethodPost, "/api/reservations/hold", map[string]any{
		"zone_id":           fx.zoneID,
		"member_id":         fx.memberID,
		"vehicle_id":        fx.vehicleID,
		"time_window_start": start.Format(time.RFC3339),
		"time_window_end":   end.Format(time.RFC3339),
		"stall_count":       1,
	}, admin)
	logStep(t, "POST", "/api/reservations/hold", hold.Code, hold.Body.String())
	if hold.Code != http.StatusCreated {
		t.Fatalf("create hold failed: %d %s", hold.Code, hold.Body.String())
	}
	reservationID := extractID(t, hold.Body.String())

	_, err := env.pool.Exec(context.Background(), `UPDATE capacity_holds SET expires_at = now() - interval '1 minute' WHERE reservation_id = $1`, reservationID)
	if err != nil {
		t.Fatalf("expire hold row: %v", err)
	}

	confirm := apiRequest(t, env.r, http.MethodPost, "/api/reservations/"+reservationID+"/confirm", nil, admin)
	logStep(t, "POST", "/api/reservations/:id/confirm", confirm.Code, confirm.Body.String())
	if confirm.Code != http.StatusConflict {
		t.Fatalf("expected expired hold confirm conflict, got %d %s", confirm.Code, confirm.Body.String())
	}
}

func TestCreateHoldOversellAndConcurrentOversell(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fx := createReservationFixture(t, env, admin, 1, 15)

	start := time.Now().UTC().Add(4 * time.Hour).Truncate(time.Second)
	end := start.Add(2 * time.Hour)

	first := apiRequest(t, env.r, http.MethodPost, "/api/reservations/hold", map[string]any{
		"zone_id":           fx.zoneID,
		"member_id":         fx.memberID,
		"vehicle_id":        fx.vehicleID,
		"time_window_start": start.Format(time.RFC3339),
		"time_window_end":   end.Format(time.RFC3339),
		"stall_count":       1,
	}, admin)
	logStep(t, "POST", "/api/reservations/hold", first.Code, first.Body.String())
	if first.Code != http.StatusCreated {
		t.Fatalf("first hold failed: %d %s", first.Code, first.Body.String())
	}

	second := apiRequest(t, env.r, http.MethodPost, "/api/reservations/hold", map[string]any{
		"zone_id":           fx.zoneID,
		"member_id":         fx.memberID,
		"vehicle_id":        fx.vehicleID,
		"time_window_start": start.Format(time.RFC3339),
		"time_window_end":   end.Format(time.RFC3339),
		"stall_count":       1,
	}, admin)
	logStep(t, "POST", "/api/reservations/hold", second.Code, second.Body.String())
	if second.Code != http.StatusConflict {
		t.Fatalf("expected second hold conflict, got %d %s", second.Code, second.Body.String())
	}

	// New fixture for true parallel race.
	env2 := setupAuthAPIEnv(t)
	admin2 := loginAs(t, env2, "admin", "AdminPass1234")
	fx2 := createReservationFixture(t, env2, admin2, 1, 15)
	start2 := time.Now().UTC().Add(5 * time.Hour).Truncate(time.Second)
	end2 := start2.Add(2 * time.Hour)

	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := apiRequest(t, env2.r, http.MethodPost, "/api/reservations/hold", map[string]any{
				"zone_id":           fx2.zoneID,
				"member_id":         fx2.memberID,
				"vehicle_id":        fx2.vehicleID,
				"time_window_start": start2.Format(time.RFC3339),
				"time_window_end":   end2.Format(time.RFC3339),
				"stall_count":       1,
			}, admin2)
			codes <- w.Code
		}()
	}
	wg.Wait()
	close(codes)

	created := 0
	conflicts := 0
	for code := range codes {
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		}
	}
	if created != 1 || conflicts != 1 {
		t.Fatalf("expected one create and one conflict, got created=%d conflicts=%d", created, conflicts)
	}
}

func TestZoneStallReductionBlockedBelowConfirmedDemand(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fx := createReservationFixture(t, env, admin, 3, 15)

	start := time.Now().UTC().Add(6 * time.Hour).Truncate(time.Second)
	end := start.Add(2 * time.Hour)

	hold := apiRequest(t, env.r, http.MethodPost, "/api/reservations/hold", map[string]any{
		"zone_id":           fx.zoneID,
		"member_id":         fx.memberID,
		"vehicle_id":        fx.vehicleID,
		"time_window_start": start.Format(time.RFC3339),
		"time_window_end":   end.Format(time.RFC3339),
		"stall_count":       2,
	}, admin)
	logStep(t, "POST", "/api/reservations/hold", hold.Code, hold.Body.String())
	if hold.Code != http.StatusCreated {
		t.Fatalf("hold failed: %d %s", hold.Code, hold.Body.String())
	}
	reservationID := extractID(t, hold.Body.String())

	confirm := apiRequest(t, env.r, http.MethodPost, "/api/reservations/"+reservationID+"/confirm", nil, admin)
	logStep(t, "POST", "/api/reservations/:id/confirm", confirm.Code, confirm.Body.String())
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm failed: %d %s", confirm.Code, confirm.Body.String())
	}

	patchZone := apiRequest(t, env.r, http.MethodPatch, "/api/zones/"+fx.zoneID, map[string]any{"total_stalls": 1}, admin)
	logStep(t, "PATCH", "/api/zones/:id", patchZone.Code, patchZone.Body.String())
	if patchZone.Code != http.StatusConflict {
		t.Fatalf("expected zone reduction conflict, got %d %s", patchZone.Code, patchZone.Body.String())
	}
}

func TestReservationCalendarPageRenders(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	w := apiRequest(t, env.r, http.MethodGet, "/reservations", nil, admin)
	logStep(t, "GET", "/reservations", w.Code, w.Body.String())
	if w.Code != http.StatusOK {
		t.Fatalf("expected reservation page 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Conflict warning") {
		t.Fatalf("expected conflict warning text on reservations page")
	}
}

func TestReservationTimelineFleetCrossOrgForbidden(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fleet := loginAs(t, env, "fleet", "UserPass1234")
	fx := createReservationFixture(t, env, admin, 2, 15)

	start := time.Now().UTC().Add(7 * time.Hour).Truncate(time.Second)
	end := start.Add(time.Hour)

	hold := apiRequest(t, env.r, http.MethodPost, "/api/reservations/hold", map[string]any{
		"zone_id":           fx.zoneID,
		"member_id":         fx.memberID,
		"vehicle_id":        fx.vehicleID,
		"time_window_start": start.Format(time.RFC3339),
		"time_window_end":   end.Format(time.RFC3339),
		"stall_count":       1,
	}, admin)
	if hold.Code != http.StatusCreated {
		t.Fatalf("hold failed: %d %s", hold.Code, hold.Body.String())
	}
	reservationID := extractID(t, hold.Body.String())

	timeline := apiRequest(t, env.r, http.MethodGet, "/api/reservations/"+reservationID+"/timeline", nil, fleet)
	logStep(t, "GET", "/api/reservations/:id/timeline", timeline.Code, timeline.Body.String())
	if timeline.Code != http.StatusForbidden {
		t.Fatalf("expected fleet cross-org timeline forbidden, got %d %s", timeline.Code, timeline.Body.String())
	}
}

func TestAvailabilityValidationErrors(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	w := apiRequest(t, env.r, http.MethodGet, "/api/availability?zone_id=x&time_window_start=bad&time_window_end=bad", nil, admin)
	logStep(t, "GET", "/api/availability", w.Code, w.Body.String())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad time inputs, got %d", w.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if _, ok := payload["code"].(string); !ok {
		t.Fatalf("expected error code in payload: %s", w.Body.String())
	}
	if _, ok := payload["message"].(string); !ok {
		t.Fatalf("expected error message in payload: %s", w.Body.String())
	}
}

func TestCapacitySnapshotLimit(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	w := apiRequest(t, env.r, http.MethodGet, "/api/capacity/snapshots?limit="+strconv.Itoa(9999), nil, admin)
	logStep(t, "GET", "/api/capacity/snapshots", w.Code, w.Body.String())
	if w.Code != http.StatusOK {
		t.Fatalf("expected snapshots endpoint 200, got %d", w.Code)
	}
}

func TestListHoldsAndExceptionsEndpoints(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fx := createReservationFixture(t, env, admin, 2, 15)
	start := time.Now().UTC().Add(8 * time.Hour).Truncate(time.Second)
	end := start.Add(time.Hour)

	hold := apiRequest(t, env.r, http.MethodPost, "/api/reservations/hold", map[string]any{
		"zone_id":           fx.zoneID,
		"member_id":         fx.memberID,
		"vehicle_id":        fx.vehicleID,
		"time_window_start": start.Format(time.RFC3339),
		"time_window_end":   end.Format(time.RFC3339),
		"stall_count":       1,
	}, admin)
	if hold.Code != http.StatusCreated {
		t.Fatalf("create hold failed: %d %s", hold.Code, hold.Body.String())
	}

	holds := apiRequest(t, env.r, http.MethodGet, "/api/reservations?status=hold", nil, admin)
	logStep(t, "GET", "/api/reservations?status=hold", holds.Code, holds.Body.String())
	if holds.Code != http.StatusOK || !strings.Contains(holds.Body.String(), "hold_expires_at") {
		t.Fatalf("expected holds list with hold_expires_at, got %d %s", holds.Code, holds.Body.String())
	}

	ex := apiRequest(t, env.r, http.MethodGet, "/api/exceptions", nil, admin)
	logStep(t, "GET", "/api/exceptions", ex.Code, ex.Body.String())
	if ex.Code != http.StatusOK || !strings.Contains(ex.Body.String(), `"items"`) {
		t.Fatalf("expected exceptions items response, got %d %s", ex.Code, ex.Body.String())
	}
}
