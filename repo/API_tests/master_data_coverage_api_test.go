package API_tests

// These tests close the gaps called out in the test coverage audit for
// master-data GET/PATCH/DELETE endpoints that previously had no direct HTTP
// test. Each test exercises a full happy-path plus key failure/permission
// checks through the production router (no mocks).

import (
	"net/http"
	"strings"
	"testing"
)

func TestFacilityReadUpdateDelete(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	dispatch := loginAs(t, env, "operator", "UserPass1234")

	// Seed facility
	create := apiRequest(t, env.r, http.MethodPost, "/api/facilities", map[string]any{"name": "F-Cov", "address": "100 Main"}, admin)
	if create.Code != http.StatusCreated {
		t.Fatalf("create facility: %d %s", create.Code, create.Body.String())
	}
	id := extractID(t, create.Body.String())

	// GET /api/facilities (list) — covered endpoint
	list := apiRequest(t, env.r, http.MethodGet, "/api/facilities", nil, admin)
	logStep(t, "GET", "/api/facilities", list.Code, list.Body.String())
	if list.Code != http.StatusOK {
		t.Fatalf("list facilities: %d %s", list.Code, list.Body.String())
	}
	if !strings.Contains(list.Body.String(), id) {
		t.Fatalf("list facilities missing id %s: %s", id, list.Body.String())
	}

	// GET /api/facilities/:id
	get := apiRequest(t, env.r, http.MethodGet, "/api/facilities/"+id, nil, admin)
	logStep(t, "GET", "/api/facilities/:id", get.Code, get.Body.String())
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), "F-Cov") {
		t.Fatalf("get facility: %d %s", get.Code, get.Body.String())
	}

	// PATCH /api/facilities/:id
	patch := apiRequest(t, env.r, http.MethodPatch, "/api/facilities/"+id, map[string]any{"name": "F-Cov-Renamed"}, admin)
	logStep(t, "PATCH", "/api/facilities/:id", patch.Code, patch.Body.String())
	if patch.Code != http.StatusOK {
		t.Fatalf("patch facility: %d %s", patch.Code, patch.Body.String())
	}
	getAfter := apiRequest(t, env.r, http.MethodGet, "/api/facilities/"+id, nil, admin)
	if !strings.Contains(getAfter.Body.String(), "F-Cov-Renamed") {
		t.Fatalf("patch did not persist: %s", getAfter.Body.String())
	}

	// PATCH as dispatch (non-admin) -> 403
	forbiddenPatch := apiRequest(t, env.r, http.MethodPatch, "/api/facilities/"+id, map[string]any{"name": "Denied"}, dispatch)
	if forbiddenPatch.Code != http.StatusForbidden {
		t.Fatalf("expected dispatch patch forbidden, got %d", forbiddenPatch.Code)
	}

	// DELETE as dispatch -> 403
	forbiddenDelete := apiRequest(t, env.r, http.MethodDelete, "/api/facilities/"+id, nil, dispatch)
	if forbiddenDelete.Code != http.StatusForbidden {
		t.Fatalf("expected dispatch delete forbidden, got %d", forbiddenDelete.Code)
	}

	// DELETE as admin
	del := apiRequest(t, env.r, http.MethodDelete, "/api/facilities/"+id, nil, admin)
	logStep(t, "DELETE", "/api/facilities/:id", del.Code, del.Body.String())
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete facility: %d %s", del.Code, del.Body.String())
	}
	getDeleted := apiRequest(t, env.r, http.MethodGet, "/api/facilities/"+id, nil, admin)
	if getDeleted.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", getDeleted.Code)
	}
}

func TestLotReadUpdateDelete(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	fac := apiRequest(t, env.r, http.MethodPost, "/api/facilities", map[string]any{"name": "FL", "address": "a"}, admin)
	if fac.Code != http.StatusCreated {
		t.Fatalf("seed facility: %d %s", fac.Code, fac.Body.String())
	}
	facID := extractID(t, fac.Body.String())

	lot := apiRequest(t, env.r, http.MethodPost, "/api/lots", map[string]any{"facility_id": facID, "name": "L-Cov"}, admin)
	if lot.Code != http.StatusCreated {
		t.Fatalf("create lot: %d %s", lot.Code, lot.Body.String())
	}
	lotID := extractID(t, lot.Body.String())

	// GET /api/lots (list)
	list := apiRequest(t, env.r, http.MethodGet, "/api/lots", nil, admin)
	logStep(t, "GET", "/api/lots", list.Code, list.Body.String())
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), lotID) {
		t.Fatalf("list lots: %d %s", list.Code, list.Body.String())
	}

	// GET /api/lots?facility_id=X
	scopedList := apiRequest(t, env.r, http.MethodGet, "/api/lots?facility_id="+facID, nil, admin)
	if scopedList.Code != http.StatusOK || !strings.Contains(scopedList.Body.String(), lotID) {
		t.Fatalf("scoped lots list: %d %s", scopedList.Code, scopedList.Body.String())
	}

	// GET /api/lots/:id
	get := apiRequest(t, env.r, http.MethodGet, "/api/lots/"+lotID, nil, admin)
	logStep(t, "GET", "/api/lots/:id", get.Code, get.Body.String())
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), "L-Cov") {
		t.Fatalf("get lot: %d %s", get.Code, get.Body.String())
	}

	// PATCH /api/lots/:id
	patch := apiRequest(t, env.r, http.MethodPatch, "/api/lots/"+lotID, map[string]any{"name": "L-Cov-Renamed"}, admin)
	logStep(t, "PATCH", "/api/lots/:id", patch.Code, patch.Body.String())
	if patch.Code != http.StatusOK {
		t.Fatalf("patch lot: %d %s", patch.Code, patch.Body.String())
	}

	// DELETE /api/lots/:id
	del := apiRequest(t, env.r, http.MethodDelete, "/api/lots/"+lotID, nil, admin)
	logStep(t, "DELETE", "/api/lots/:id", del.Code, del.Body.String())
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete lot: %d %s", del.Code, del.Body.String())
	}
}

func TestZoneReadDelete(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	fac := apiRequest(t, env.r, http.MethodPost, "/api/facilities", map[string]any{"name": "FZ", "address": "z"}, admin)
	facID := extractID(t, fac.Body.String())
	lot := apiRequest(t, env.r, http.MethodPost, "/api/lots", map[string]any{"facility_id": facID, "name": "LZ"}, admin)
	lotID := extractID(t, lot.Body.String())
	zone := apiRequest(t, env.r, http.MethodPost, "/api/zones", map[string]any{"lot_id": lotID, "name": "Z-Cov", "total_stalls": 20}, admin)
	if zone.Code != http.StatusCreated {
		t.Fatalf("create zone: %d %s", zone.Code, zone.Body.String())
	}
	zoneID := extractID(t, zone.Body.String())

	// GET /api/zones/:id
	get := apiRequest(t, env.r, http.MethodGet, "/api/zones/"+zoneID, nil, admin)
	logStep(t, "GET", "/api/zones/:id", get.Code, get.Body.String())
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), "Z-Cov") {
		t.Fatalf("get zone: %d %s", get.Code, get.Body.String())
	}
	if !strings.Contains(get.Body.String(), `"total_stalls":20`) {
		t.Fatalf("zone body missing total_stalls: %s", get.Body.String())
	}

	// DELETE /api/zones/:id
	del := apiRequest(t, env.r, http.MethodDelete, "/api/zones/"+zoneID, nil, admin)
	logStep(t, "DELETE", "/api/zones/:id", del.Code, del.Body.String())
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete zone: %d %s", del.Code, del.Body.String())
	}
	// GET after delete should be 404
	getAfter := apiRequest(t, env.r, http.MethodGet, "/api/zones/"+zoneID, nil, admin)
	if getAfter.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after zone delete, got %d", getAfter.Code)
	}
}

func TestRatePlanReadUpdateDelete(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	fac := apiRequest(t, env.r, http.MethodPost, "/api/facilities", map[string]any{"name": "FR", "address": "r"}, admin)
	facID := extractID(t, fac.Body.String())
	lot := apiRequest(t, env.r, http.MethodPost, "/api/lots", map[string]any{"facility_id": facID, "name": "LR"}, admin)
	lotID := extractID(t, lot.Body.String())
	zone := apiRequest(t, env.r, http.MethodPost, "/api/zones", map[string]any{"lot_id": lotID, "name": "ZR", "total_stalls": 10}, admin)
	zoneID := extractID(t, zone.Body.String())
	rate := apiRequest(t, env.r, http.MethodPost, "/api/rate-plans", map[string]any{"zone_id": zoneID, "name": "Hourly", "rate_cents": 500, "period": "hourly"}, admin)
	if rate.Code != http.StatusCreated {
		t.Fatalf("create rate plan: %d %s", rate.Code, rate.Body.String())
	}
	rateID := extractID(t, rate.Body.String())

	// GET /api/rate-plans (list)
	list := apiRequest(t, env.r, http.MethodGet, "/api/rate-plans", nil, admin)
	logStep(t, "GET", "/api/rate-plans", list.Code, list.Body.String())
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), rateID) {
		t.Fatalf("list rate-plans: %d %s", list.Code, list.Body.String())
	}

	// GET scoped by zone_id
	scoped := apiRequest(t, env.r, http.MethodGet, "/api/rate-plans?zone_id="+zoneID, nil, admin)
	if scoped.Code != http.StatusOK || !strings.Contains(scoped.Body.String(), rateID) {
		t.Fatalf("scoped rate-plans list: %d %s", scoped.Code, scoped.Body.String())
	}

	// GET /api/rate-plans/:id
	get := apiRequest(t, env.r, http.MethodGet, "/api/rate-plans/"+rateID, nil, admin)
	logStep(t, "GET", "/api/rate-plans/:id", get.Code, get.Body.String())
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), "Hourly") {
		t.Fatalf("get rate plan: %d %s", get.Code, get.Body.String())
	}

	// PATCH /api/rate-plans/:id
	patch := apiRequest(t, env.r, http.MethodPatch, "/api/rate-plans/"+rateID, map[string]any{"name": "Hourly-Updated", "rate_cents": 750, "period": "hourly"}, admin)
	logStep(t, "PATCH", "/api/rate-plans/:id", patch.Code, patch.Body.String())
	if patch.Code != http.StatusOK {
		t.Fatalf("patch rate plan: %d %s", patch.Code, patch.Body.String())
	}
	getAfter := apiRequest(t, env.r, http.MethodGet, "/api/rate-plans/"+rateID, nil, admin)
	if !strings.Contains(getAfter.Body.String(), "Hourly-Updated") || !strings.Contains(getAfter.Body.String(), `"rate_cents":750`) {
		t.Fatalf("patch did not persist: %s", getAfter.Body.String())
	}

	// DELETE /api/rate-plans/:id
	del := apiRequest(t, env.r, http.MethodDelete, "/api/rate-plans/"+rateID, nil, admin)
	logStep(t, "DELETE", "/api/rate-plans/:id", del.Code, del.Body.String())
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete rate plan: %d %s", del.Code, del.Body.String())
	}
}

func TestMessageRuleReadUpdateDelete(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	// Discover a topic ID
	topics := apiRequest(t, env.r, http.MethodGet, "/api/notification-topics", nil, admin)
	if topics.Code != http.StatusOK {
		t.Fatalf("topics: %d %s", topics.Code, topics.Body.String())
	}
	topicID := topicIDByName(t, topics.Body.String(), "booking_success")

	create := apiRequest(t, env.r, http.MethodPost, "/api/message-rules", map[string]any{
		"trigger_event": "booking.confirmed",
		"topic_id":      topicID,
		"template":      "Hello {{.member}}",
		"active":        true,
	}, admin)
	if create.Code != http.StatusCreated {
		t.Fatalf("create message rule: %d %s", create.Code, create.Body.String())
	}
	ruleID := extractID(t, create.Body.String())

	// GET /api/message-rules (list)
	list := apiRequest(t, env.r, http.MethodGet, "/api/message-rules", nil, admin)
	logStep(t, "GET", "/api/message-rules", list.Code, list.Body.String())
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), ruleID) {
		t.Fatalf("list message rules: %d %s", list.Code, list.Body.String())
	}
	if !strings.Contains(list.Body.String(), "booking.confirmed") {
		t.Fatalf("list missing trigger_event: %s", list.Body.String())
	}

	// PATCH /api/message-rules/:id — toggle active off
	inactive := false
	patch := apiRequest(t, env.r, http.MethodPatch, "/api/message-rules/"+ruleID, map[string]any{"template": "Updated body", "active": inactive}, admin)
	logStep(t, "PATCH", "/api/message-rules/:id", patch.Code, patch.Body.String())
	if patch.Code != http.StatusOK {
		t.Fatalf("patch message rule: %d %s", patch.Code, patch.Body.String())
	}
	listAfter := apiRequest(t, env.r, http.MethodGet, "/api/message-rules", nil, admin)
	if !strings.Contains(listAfter.Body.String(), "Updated body") {
		t.Fatalf("patch did not persist: %s", listAfter.Body.String())
	}

	// DELETE /api/message-rules/:id
	del := apiRequest(t, env.r, http.MethodDelete, "/api/message-rules/"+ruleID, nil, admin)
	logStep(t, "DELETE", "/api/message-rules/:id", del.Code, del.Body.String())
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete message rule: %d %s", del.Code, del.Body.String())
	}
	listGone := apiRequest(t, env.r, http.MethodGet, "/api/message-rules", nil, admin)
	if strings.Contains(listGone.Body.String(), ruleID) {
		t.Fatalf("deleted message rule still listed: %s", listGone.Body.String())
	}
}
