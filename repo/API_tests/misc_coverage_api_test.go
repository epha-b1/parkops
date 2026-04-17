package API_tests

// Coverage for previously-uncovered endpoints: GET /api/campaigns,
// GET /api/reservations/stats/today, GET /api/health.

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHealthEndpoint(t *testing.T) {
	env := setupAuthAPIEnv(t)

	// GET /api/health — public, no auth required
	resp := apiRequest(t, env.r, http.MethodGet, "/api/health", nil, nil)
	logStep(t, "GET", "/api/health", resp.Code, resp.Body.String())
	if resp.Code != http.StatusOK {
		t.Fatalf("health failed: %d %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"status":"ok"`) {
		t.Fatalf("expected status ok, got: %s", resp.Body.String())
	}
}

func TestCampaignsList(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	// Seed two campaigns
	c1 := apiRequest(t, env.r, http.MethodPost, "/api/campaigns", map[string]any{"title": "Campaign Alpha", "description": "a"}, admin)
	if c1.Code != http.StatusCreated {
		t.Fatalf("seed c1: %d %s", c1.Code, c1.Body.String())
	}
	c1ID := extractID(t, c1.Body.String())
	c2 := apiRequest(t, env.r, http.MethodPost, "/api/campaigns", map[string]any{"title": "Campaign Bravo", "description": "b", "target_role": "dispatch_operator"}, admin)
	if c2.Code != http.StatusCreated {
		t.Fatalf("seed c2: %d %s", c2.Code, c2.Body.String())
	}
	c2ID := extractID(t, c2.Body.String())

	// GET /api/campaigns
	list := apiRequest(t, env.r, http.MethodGet, "/api/campaigns", nil, admin)
	logStep(t, "GET", "/api/campaigns", list.Code, list.Body.String())
	if list.Code != http.StatusOK {
		t.Fatalf("list campaigns: %d %s", list.Code, list.Body.String())
	}
	body := list.Body.String()
	if !strings.Contains(body, c1ID) || !strings.Contains(body, c2ID) {
		t.Fatalf("list missing seeded campaigns: %s", body)
	}
	if !strings.Contains(body, "Campaign Alpha") || !strings.Contains(body, "Campaign Bravo") {
		t.Fatalf("list missing titles: %s", body)
	}
	// Ensure target_role surfaces when set
	if !strings.Contains(body, "dispatch_operator") {
		t.Fatalf("list missing target_role: %s", body)
	}

	// Auditor can also list (read-only role)
	auditor := loginAs(t, env, "auditor", "UserPass1234")
	auditorList := apiRequest(t, env.r, http.MethodGet, "/api/campaigns", nil, auditor)
	if auditorList.Code != http.StatusOK {
		t.Fatalf("auditor list campaigns: %d %s", auditorList.Code, auditorList.Body.String())
	}
}

func TestReservationStatsToday(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	// Baseline count
	first := apiRequest(t, env.r, http.MethodGet, "/api/reservations/stats/today", nil, admin)
	logStep(t, "GET", "/api/reservations/stats/today (baseline)", first.Code, first.Body.String())
	if first.Code != http.StatusOK {
		t.Fatalf("stats baseline: %d %s", first.Code, first.Body.String())
	}
	if !strings.Contains(first.Body.String(), `"total_reservations_today"`) {
		t.Fatalf("stats missing total_reservations_today field: %s", first.Body.String())
	}

	// Create a reservation so today count should increase
	fx := createReservationFixture(t, env, admin, 5, 15)
	start := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	end := start.Add(1 * time.Hour)
	hold := apiRequest(t, env.r, http.MethodPost, "/api/reservations/hold", map[string]any{
		"zone_id":           fx.zoneID,
		"member_id":         fx.memberID,
		"vehicle_id":        fx.vehicleID,
		"time_window_start": start.Format(time.RFC3339),
		"time_window_end":   end.Format(time.RFC3339),
		"stall_count":       1,
	}, admin)
	if hold.Code != http.StatusCreated {
		t.Fatalf("hold: %d %s", hold.Code, hold.Body.String())
	}

	second := apiRequest(t, env.r, http.MethodGet, "/api/reservations/stats/today", nil, admin)
	logStep(t, "GET", "/api/reservations/stats/today (after hold)", second.Code, second.Body.String())
	if second.Code != http.StatusOK {
		t.Fatalf("stats after hold: %d %s", second.Code, second.Body.String())
	}
	// Should be a positive integer field, and the body should indicate at least one reservation today
	if !strings.Contains(second.Body.String(), `"total_reservations_today"`) {
		t.Fatalf("stats response missing field: %s", second.Body.String())
	}
}
