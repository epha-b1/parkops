package API_tests

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestFleetCrossOrgReadScopes(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fleet := loginAs(t, env, "fleet", "UserPass1234")

	fx := createReservationFixture(t, env, admin, 2, 15)

	createTag := apiRequest(t, env.r, http.MethodPost, "/api/tags", map[string]any{
		"name": "scope_tag",
	}, admin)
	logStep(t, "POST", "/api/tags", createTag.Code, createTag.Body.String())
	if createTag.Code != http.StatusCreated {
		t.Fatalf("create tag failed: %d %s", createTag.Code, createTag.Body.String())
	}
	tagID := extractID(t, createTag.Body.String())

	addTag := apiRequest(t, env.r, http.MethodPost, "/api/members/"+fx.memberID+"/tags", map[string]any{
		"tag_id": tagID,
	}, admin)
	logStep(t, "POST", "/api/members/:id/tags", addTag.Code, addTag.Body.String())
	if addTag.Code != http.StatusOK {
		t.Fatalf("add tag failed: %d %s", addTag.Code, addTag.Body.String())
	}

	start := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	end := start.Add(2 * time.Hour)
	res := apiRequest(t, env.r, http.MethodPost, "/api/reservations/hold", map[string]any{
		"zone_id":           fx.zoneID,
		"member_id":         fx.memberID,
		"vehicle_id":        fx.vehicleID,
		"stall_count":       1,
		"time_window_start": start.Format(time.RFC3339),
		"time_window_end":   end.Format(time.RFC3339),
	}, admin)
	logStep(t, "POST", "/api/reservations/hold", res.Code, res.Body.String())
	if res.Code != http.StatusCreated {
		t.Fatalf("create hold failed: %d %s", res.Code, res.Body.String())
	}
	reservationID := extractID(t, res.Body.String())

	list := apiRequest(t, env.r, http.MethodGet, "/api/reservations", nil, fleet)
	logStep(t, "GET", "/api/reservations (fleet)", list.Code, list.Body.String())
	if list.Code != http.StatusOK {
		t.Fatalf("list reservations failed: %d %s", list.Code, list.Body.String())
	}
	if strings.Contains(list.Body.String(), reservationID) {
		t.Fatalf("fleet should not see cross-org reservations")
	}

	deviceID, _ := createDeviceWithZone(t, env, admin)
	listDevices := apiRequest(t, env.r, http.MethodGet, "/api/devices", nil, fleet)
	logStep(t, "GET", "/api/devices (fleet)", listDevices.Code, listDevices.Body.String())
	if listDevices.Code != http.StatusOK {
		t.Fatalf("list devices failed: %d %s", listDevices.Code, listDevices.Body.String())
	}
	if strings.Contains(listDevices.Body.String(), deviceID) {
		t.Fatalf("fleet should not see cross-org devices")
	}

	getDevice := apiRequest(t, env.r, http.MethodGet, "/api/devices/"+deviceID, nil, fleet)
	logStep(t, "GET", "/api/devices/:id (fleet)", getDevice.Code, getDevice.Body.String())
	if getDevice.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for fleet get device, got %d", getDevice.Code)
	}

	loc := apiRequest(t, env.r, http.MethodPost, "/api/tracking/location", map[string]any{
		"vehicle_id": fx.vehicleID,
		"latitude":   37.7749,
		"longitude":  -122.4194,
	}, admin)
	logStep(t, "POST", "/api/tracking/location", loc.Code, loc.Body.String())
	if loc.Code != http.StatusCreated {
		t.Fatalf("submit location failed: %d %s", loc.Code, loc.Body.String())
	}

	positions := apiRequest(t, env.r, http.MethodGet, "/api/tracking/vehicles/"+fx.vehicleID+"/positions", nil, fleet)
	logStep(t, "GET", "/api/tracking/vehicles/:id/positions (fleet)", positions.Code, positions.Body.String())
	if positions.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for fleet positions, got %d", positions.Code)
	}

	stops := apiRequest(t, env.r, http.MethodGet, "/api/tracking/vehicles/"+fx.vehicleID+"/stops", nil, fleet)
	logStep(t, "GET", "/api/tracking/vehicles/:id/stops (fleet)", stops.Code, stops.Body.String())
	if stops.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for fleet stops, got %d", stops.Code)
	}

	memberTags := apiRequest(t, env.r, http.MethodGet, "/api/members/"+fx.memberID+"/tags", nil, fleet)
	logStep(t, "GET", "/api/members/:id/tags (fleet)", memberTags.Code, memberTags.Body.String())
	if memberTags.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for fleet member tags, got %d", memberTags.Code)
	}

	addCrossOrgTag := apiRequest(t, env.r, http.MethodPost, "/api/members/"+fx.memberID+"/tags", map[string]any{
		"tag_id": tagID,
	}, fleet)
	logStep(t, "POST", "/api/members/:id/tags (fleet)", addCrossOrgTag.Code, addCrossOrgTag.Body.String())
	if addCrossOrgTag.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for fleet add member tag, got %d", addCrossOrgTag.Code)
	}

	removeCrossOrgTag := apiRequest(t, env.r, http.MethodDelete, "/api/members/"+fx.memberID+"/tags/"+tagID, nil, fleet)
	logStep(t, "DELETE", "/api/members/:id/tags/:tagId (fleet)", removeCrossOrgTag.Code, removeCrossOrgTag.Body.String())
	if removeCrossOrgTag.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for fleet remove member tag, got %d", removeCrossOrgTag.Code)
	}
}
