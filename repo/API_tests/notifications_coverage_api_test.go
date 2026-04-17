package API_tests

// Coverage for notification-settings generic GET/PATCH and the unsubscribe
// endpoint which were previously uncovered by direct HTTP tests.

import (
	"net/http"
	"strings"
	"testing"
)

func TestNotificationSettingsGenericGetAndPatch(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	// GET /api/notification-settings — returns default DND envelope when none saved
	get := apiRequest(t, env.r, http.MethodGet, "/api/notification-settings", nil, admin)
	logStep(t, "GET", "/api/notification-settings", get.Code, get.Body.String())
	if get.Code != http.StatusOK {
		t.Fatalf("get settings: %d %s", get.Code, get.Body.String())
	}
	if !strings.Contains(get.Body.String(), `"dnd"`) {
		t.Fatalf("settings body missing dnd envelope: %s", get.Body.String())
	}

	// PATCH /api/notification-settings (generic alias of PATCH /dnd)
	patch := apiRequest(t, env.r, http.MethodPatch, "/api/notification-settings", map[string]any{
		"dnd": map[string]any{
			"start_time": "21:00",
			"end_time":   "06:30",
			"enabled":    true,
		},
	}, admin)
	logStep(t, "PATCH", "/api/notification-settings", patch.Code, patch.Body.String())
	if patch.Code != http.StatusOK {
		t.Fatalf("patch settings: %d %s", patch.Code, patch.Body.String())
	}
	if !strings.Contains(patch.Body.String(), "21:00") || !strings.Contains(patch.Body.String(), "06:30") {
		t.Fatalf("patch settings response missing times: %s", patch.Body.String())
	}

	// GET after PATCH — verify persistence
	getAfter := apiRequest(t, env.r, http.MethodGet, "/api/notification-settings", nil, admin)
	if !strings.Contains(getAfter.Body.String(), `"enabled":true`) {
		t.Fatalf("persisted settings missing enabled=true: %s", getAfter.Body.String())
	}

	// Validation: missing times -> 400
	bad := apiRequest(t, env.r, http.MethodPatch, "/api/notification-settings", map[string]any{"dnd": map[string]any{"enabled": true}}, admin)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing times, got %d", bad.Code)
	}
}

func TestNotificationTopicUnsubscribe(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	topics := apiRequest(t, env.r, http.MethodGet, "/api/notification-topics", nil, admin)
	if topics.Code != http.StatusOK {
		t.Fatalf("topics: %d %s", topics.Code, topics.Body.String())
	}
	topicID := topicIDByName(t, topics.Body.String(), "booking_success")

	// Subscribe first
	sub := apiRequest(t, env.r, http.MethodPost, "/api/notification-topics/"+topicID+"/subscribe", nil, admin)
	if sub.Code != http.StatusOK {
		t.Fatalf("subscribe: %d %s", sub.Code, sub.Body.String())
	}

	// Verify subscribed=true
	after := apiRequest(t, env.r, http.MethodGet, "/api/notification-topics", nil, admin)
	if !strings.Contains(after.Body.String(), `"subscribed":true`) {
		t.Fatalf("expected subscribed=true after subscribe: %s", after.Body.String())
	}

	// DELETE /api/notification-topics/:id/subscribe
	unsub := apiRequest(t, env.r, http.MethodDelete, "/api/notification-topics/"+topicID+"/subscribe", nil, admin)
	logStep(t, "DELETE", "/api/notification-topics/:id/subscribe", unsub.Code, unsub.Body.String())
	if unsub.Code != http.StatusOK {
		t.Fatalf("unsubscribe: %d %s", unsub.Code, unsub.Body.String())
	}

	// Verify the specific topic is no longer subscribed
	check := apiRequest(t, env.r, http.MethodGet, "/api/notification-topics", nil, admin)
	// The "booking_success" topic object should now show subscribed:false
	needle := `"name":"booking_success"`
	idx := strings.Index(check.Body.String(), needle)
	if idx < 0 {
		t.Fatalf("booking_success topic missing: %s", check.Body.String())
	}
	// Look at a 100-char window after the needle for "subscribed":false
	window := check.Body.String()[idx:min(idx+200, len(check.Body.String()))]
	if !strings.Contains(window, `"subscribed":false`) {
		t.Fatalf("expected subscribed=false for booking_success after unsubscribe, window: %s", window)
	}
}
