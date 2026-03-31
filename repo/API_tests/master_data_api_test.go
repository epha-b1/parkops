package API_tests

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestMasterDataCRUDHappyPathAndWrongRole(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	dispatch := loginAs(t, env, "operator", "UserPass1234")

	forbidden := apiRequest(t, env.r, http.MethodPost, "/api/facilities", map[string]any{"name": "Forbidden Facility"}, dispatch)
	logStep(t, "POST", "/api/facilities", forbidden.Code, forbidden.Body.String())
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected dispatch forbidden create facility, got %d", forbidden.Code)
	}

	fac := apiRequest(t, env.r, http.MethodPost, "/api/facilities", map[string]any{"name": "F1", "address": "Main"}, admin)
	logStep(t, "POST", "/api/facilities", fac.Code, fac.Body.String())
	if fac.Code != http.StatusCreated {
		t.Fatalf("create facility failed: %d %s", fac.Code, fac.Body.String())
	}
	facID := extractID(t, fac.Body.String())

	lot := apiRequest(t, env.r, http.MethodPost, "/api/lots", map[string]any{"facility_id": facID, "name": "Lot A"}, admin)
	logStep(t, "POST", "/api/lots", lot.Code, lot.Body.String())
	if lot.Code != http.StatusCreated {
		t.Fatalf("create lot failed: %d %s", lot.Code, lot.Body.String())
	}
	lotID := extractID(t, lot.Body.String())

	zone := apiRequest(t, env.r, http.MethodPost, "/api/zones", map[string]any{"lot_id": lotID, "name": "Z1", "total_stalls": 50}, admin)
	logStep(t, "POST", "/api/zones", zone.Code, zone.Body.String())
	if zone.Code != http.StatusCreated {
		t.Fatalf("create zone failed: %d %s", zone.Code, zone.Body.String())
	}
	zoneID := extractID(t, zone.Body.String())

	rate := apiRequest(t, env.r, http.MethodPost, "/api/rate-plans", map[string]any{"zone_id": zoneID, "name": "Hourly", "rate_cents": 500, "period": "hourly"}, admin)
	logStep(t, "POST", "/api/rate-plans", rate.Code, rate.Body.String())
	if rate.Code != http.StatusCreated {
		t.Fatalf("create rate plan failed: %d %s", rate.Code, rate.Body.String())
	}

	listZones := apiRequest(t, env.r, http.MethodGet, "/api/zones", nil, admin)
	logStep(t, "GET", "/api/zones", listZones.Code, listZones.Body.String())
	if listZones.Code != http.StatusOK || !strings.Contains(listZones.Body.String(), "hold_timeout_minutes") {
		t.Fatalf("zones list invalid: %d %s", listZones.Code, listZones.Body.String())
	}
}

func TestMembersVehiclesDriversMessageRulesAndOrgScope(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fleet := loginAs(t, env, "fleet", "UserPass1234")

	member := apiRequest(t, env.r, http.MethodPost, "/api/members", map[string]any{"display_name": "OrgA Member", "contact_notes": "VIP note"}, admin)
	logStep(t, "POST", "/api/members", member.Code, member.Body.String())
	if member.Code != http.StatusCreated {
		t.Fatalf("create member failed: %d %s", member.Code, member.Body.String())
	}
	memberID := extractID(t, member.Body.String())

	listMembers := apiRequest(t, env.r, http.MethodGet, "/api/members", nil, admin)
	logStep(t, "GET", "/api/members", listMembers.Code, listMembers.Body.String())
	if listMembers.Code != http.StatusOK {
		t.Fatalf("list members failed: %d", listMembers.Code)
	}

	getMember := apiRequest(t, env.r, http.MethodGet, "/api/members/"+memberID, nil, admin)
	logStep(t, "GET", "/api/members/:id", getMember.Code, getMember.Body.String())
	if getMember.Code != http.StatusOK {
		t.Fatalf("get member failed: %d", getMember.Code)
	}

	patchMember := apiRequest(t, env.r, http.MethodPatch, "/api/members/"+memberID, map[string]any{"display_name": "OrgA Member Updated"}, admin)
	logStep(t, "PATCH", "/api/members/:id", patchMember.Code, patchMember.Body.String())
	if patchMember.Code != http.StatusOK {
		t.Fatalf("patch member failed: %d", patchMember.Code)
	}

	veh := apiRequest(t, env.r, http.MethodPost, "/api/vehicles", map[string]any{"plate_number": "ABC123", "make": "Ford", "model": "Focus"}, admin)
	logStep(t, "POST", "/api/vehicles", veh.Code, veh.Body.String())
	if veh.Code != http.StatusCreated {
		t.Fatalf("create vehicle failed: %d %s", veh.Code, veh.Body.String())
	}
	vehicleID := extractID(t, veh.Body.String())

	listVehicles := apiRequest(t, env.r, http.MethodGet, "/api/vehicles", nil, admin)
	logStep(t, "GET", "/api/vehicles", listVehicles.Code, listVehicles.Body.String())
	if listVehicles.Code != http.StatusOK {
		t.Fatalf("list vehicles failed: %d", listVehicles.Code)
	}

	getVehicle := apiRequest(t, env.r, http.MethodGet, "/api/vehicles/"+vehicleID, nil, admin)
	logStep(t, "GET", "/api/vehicles/:id", getVehicle.Code, getVehicle.Body.String())
	if getVehicle.Code != http.StatusOK {
		t.Fatalf("get vehicle failed: %d", getVehicle.Code)
	}

	patchVehicle := apiRequest(t, env.r, http.MethodPatch, "/api/vehicles/"+vehicleID, map[string]any{"model": "Fiesta"}, admin)
	logStep(t, "PATCH", "/api/vehicles/:id", patchVehicle.Code, patchVehicle.Body.String())
	if patchVehicle.Code != http.StatusOK {
		t.Fatalf("patch vehicle failed: %d", patchVehicle.Code)
	}

	driver := apiRequest(t, env.r, http.MethodPost, "/api/drivers", map[string]any{"member_id": memberID, "licence_number": "LIC-1"}, admin)
	logStep(t, "POST", "/api/drivers", driver.Code, driver.Body.String())
	if driver.Code != http.StatusCreated {
		t.Fatalf("create driver failed: %d %s", driver.Code, driver.Body.String())
	}
	driverID := extractID(t, driver.Body.String())

	listDrivers := apiRequest(t, env.r, http.MethodGet, "/api/drivers", nil, admin)
	logStep(t, "GET", "/api/drivers", listDrivers.Code, listDrivers.Body.String())
	if listDrivers.Code != http.StatusOK {
		t.Fatalf("list drivers failed: %d", listDrivers.Code)
	}

	getDriver := apiRequest(t, env.r, http.MethodGet, "/api/drivers/"+driverID, nil, admin)
	logStep(t, "GET", "/api/drivers/:id", getDriver.Code, getDriver.Body.String())
	if getDriver.Code != http.StatusOK {
		t.Fatalf("get driver failed: %d", getDriver.Code)
	}

	patchDriver := apiRequest(t, env.r, http.MethodPatch, "/api/drivers/"+driverID, map[string]any{"licence_number": "LIC-2"}, admin)
	logStep(t, "PATCH", "/api/drivers/:id", patchDriver.Code, patchDriver.Body.String())
	if patchDriver.Code != http.StatusOK {
		t.Fatalf("patch driver failed: %d", patchDriver.Code)
	}

	rule := apiRequest(t, env.r, http.MethodPost, "/api/message-rules", map[string]any{"trigger_event": "booking.confirmed", "topic_id": "11111111-1111-1111-1111-111111111111", "template": "ok", "active": true}, admin)
	logStep(t, "POST", "/api/message-rules", rule.Code, rule.Body.String())
	if rule.Code != http.StatusCreated {
		t.Fatalf("create message rule failed: %d %s", rule.Code, rule.Body.String())
	}

	balanceGet := apiRequest(t, env.r, http.MethodGet, "/api/members/"+memberID+"/balance", nil, admin)
	logStep(t, "GET", "/api/members/:id/balance", balanceGet.Code, balanceGet.Body.String())
	if balanceGet.Code != http.StatusOK {
		t.Fatalf("get balance failed: %d", balanceGet.Code)
	}

	balancePatchForbidden := apiRequest(t, env.r, http.MethodPatch, "/api/members/"+memberID+"/balance", map[string]any{"amount_cents": 100, "reason": "test"}, fleet)
	logStep(t, "PATCH", "/api/members/:id/balance", balancePatchForbidden.Code, balancePatchForbidden.Body.String())
	if balancePatchForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected fleet patch balance forbidden, got %d", balancePatchForbidden.Code)
	}

	balancePatch := apiRequest(t, env.r, http.MethodPatch, "/api/members/"+memberID+"/balance", map[string]any{"amount_cents": 100, "reason": "test"}, admin)
	logStep(t, "PATCH", "/api/members/:id/balance", balancePatch.Code, balancePatch.Body.String())
	if balancePatch.Code != http.StatusOK {
		t.Fatalf("admin patch balance failed: %d", balancePatch.Code)
	}

	var auditCount int
	var err error
	err = env.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action='member_balance_adjust' AND resource_id::text=$1`, memberID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("query member balance audit logs: %v", err)
	}
	if auditCount == 0 {
		t.Fatal("expected member balance adjustment to write audit log")
	}

	fleetCrossMember := apiRequest(t, env.r, http.MethodGet, "/api/members/"+memberID, nil, fleet)
	logStep(t, "GET", "/api/members/:id", fleetCrossMember.Code, fleetCrossMember.Body.String())
	if fleetCrossMember.Code != http.StatusForbidden {
		t.Fatalf("expected fleet cross-org member forbidden, got %d", fleetCrossMember.Code)
	}

	fleetCrossVehicle := apiRequest(t, env.r, http.MethodGet, "/api/vehicles/"+vehicleID, nil, fleet)
	logStep(t, "GET", "/api/vehicles/:id", fleetCrossVehicle.Code, fleetCrossVehicle.Body.String())
	if fleetCrossVehicle.Code != http.StatusForbidden {
		t.Fatalf("expected fleet cross-org vehicle forbidden, got %d", fleetCrossVehicle.Code)
	}

	var contactEnc string
	err = env.pool.QueryRow(context.Background(), `SELECT contact_notes_enc FROM members WHERE id = $1`, memberID).Scan(&contactEnc)
	if err != nil {
		t.Fatalf("query encrypted contact notes: %v", err)
	}
	if strings.Contains(contactEnc, "VIP note") {
		t.Fatalf("contact notes stored in plaintext: %s", contactEnc)
	}

	delDriver := apiRequest(t, env.r, http.MethodDelete, "/api/drivers/"+driverID, nil, admin)
	logStep(t, "DELETE", "/api/drivers/:id", delDriver.Code, delDriver.Body.String())
	if delDriver.Code != http.StatusNoContent {
		t.Fatalf("delete driver failed: %d", delDriver.Code)
	}

	delVehicle := apiRequest(t, env.r, http.MethodDelete, "/api/vehicles/"+vehicleID, nil, admin)
	logStep(t, "DELETE", "/api/vehicles/:id", delVehicle.Code, delVehicle.Body.String())
	if delVehicle.Code != http.StatusNoContent {
		t.Fatalf("delete vehicle failed: %d", delVehicle.Code)
	}

	delMember := apiRequest(t, env.r, http.MethodDelete, "/api/members/"+memberID, nil, admin)
	logStep(t, "DELETE", "/api/members/:id", delMember.Code, delMember.Body.String())
	if delMember.Code != http.StatusNoContent {
		t.Fatalf("delete member failed: %d", delMember.Code)
	}
}
