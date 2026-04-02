package API_tests

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func createDeviceWithZone(t *testing.T, env *apiTestEnv, admin *http.Cookie) (string, string) {
	t.Helper()
	fx := createReservationFixture(t, env, admin, 10, 15)
	resp := apiRequest(t, env.r, http.MethodPost, "/api/devices", map[string]any{
		"device_key":  "device-key-" + time.Now().UTC().Format("150405.000000"),
		"device_type": "camera",
		"zone_id":     fx.zoneID,
		"status":      "online",
	}, admin)
	logStep(t, "POST", "/api/devices", resp.Code, resp.Body.String())
	if resp.Code != http.StatusCreated {
		t.Fatalf("register device failed: %d %s", resp.Code, resp.Body.String())
	}
	return extractID(t, resp.Body.String()), fx.zoneID
}

func TestRegisterDevice(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	deviceID, _ := createDeviceWithZone(t, env, admin)

	list := apiRequest(t, env.r, http.MethodGet, "/api/devices", nil, admin)
	logStep(t, "GET", "/api/devices", list.Code, list.Body.String())
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), deviceID) {
		t.Fatalf("expected list to include device, got %d %s", list.Code, list.Body.String())
	}

	getOne := apiRequest(t, env.r, http.MethodGet, "/api/devices/"+deviceID, nil, admin)
	logStep(t, "GET", "/api/devices/:id", getOne.Code, getOne.Body.String())
	if getOne.Code != http.StatusOK || !strings.Contains(getOne.Body.String(), deviceID) {
		t.Fatalf("expected get device success, got %d %s", getOne.Code, getOne.Body.String())
	}

	patch := apiRequest(t, env.r, http.MethodPatch, "/api/devices/"+deviceID, map[string]any{"status": "offline"}, admin)
	logStep(t, "PATCH", "/api/devices/:id", patch.Code, patch.Body.String())
	if patch.Code != http.StatusOK {
		t.Fatalf("expected patch 200, got %d %s", patch.Code, patch.Body.String())
	}

	del := apiRequest(t, env.r, http.MethodDelete, "/api/devices/"+deviceID, nil, admin)
	logStep(t, "DELETE", "/api/devices/:id", del.Code, del.Body.String())
	if del.Code != http.StatusNoContent {
		t.Fatalf("expected delete 204, got %d %s", del.Code, del.Body.String())
	}
}

func TestIngestDeviceEvent(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	deviceID, _ := createDeviceWithZone(t, env, admin)

	w := apiRequest(t, env.r, http.MethodPost, "/api/device-events", map[string]any{
		"device_id":       deviceID,
		"event_key":       "ev-1",
		"sequence_number": 1,
		"event_type":      "gate_open",
		"payload":         map[string]any{"zone": "A"},
	}, admin)
	logStep(t, "POST", "/api/device-events", w.Code, w.Body.String())
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d %s", w.Code, w.Body.String())
	}
}

func TestIngestMissingDeviceID(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	w := apiRequest(t, env.r, http.MethodPost, "/api/device-events", map[string]any{
		"event_key":       "ev-missing",
		"sequence_number": 1,
		"event_type":      "gate_open",
	}, admin)
	logStep(t, "POST", "/api/device-events", w.Code, w.Body.String())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", w.Code, w.Body.String())
	}
}

func TestIngestMissingSequenceNumberAndEventKey(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	deviceID, _ := createDeviceWithZone(t, env, admin)

	missingSeq := apiRequest(t, env.r, http.MethodPost, "/api/device-events", map[string]any{
		"device_id":  deviceID,
		"event_key":  "missing-seq",
		"event_type": "gate_open",
	}, admin)
	logStep(t, "POST", "/api/device-events", missingSeq.Code, missingSeq.Body.String())
	if missingSeq.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing sequence_number, got %d %s", missingSeq.Code, missingSeq.Body.String())
	}

	missingKey := apiRequest(t, env.r, http.MethodPost, "/api/device-events", map[string]any{
		"device_id":       deviceID,
		"sequence_number": 2,
		"event_type":      "gate_open",
	}, admin)
	logStep(t, "POST", "/api/device-events", missingKey.Code, missingKey.Body.String())
	if missingKey.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing event_key, got %d %s", missingKey.Code, missingKey.Body.String())
	}
}

func TestIngestDuplicateEventKey(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	deviceID, _ := createDeviceWithZone(t, env, admin)

	first := apiRequest(t, env.r, http.MethodPost, "/api/device-events", map[string]any{
		"device_id":       deviceID,
		"event_key":       "ev-dupe",
		"sequence_number": 1,
		"event_type":      "gate_open",
	}, admin)
	logStep(t, "POST", "/api/device-events", first.Code, first.Body.String())
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first 201, got %d %s", first.Code, first.Body.String())
	}
	dupe := apiRequest(t, env.r, http.MethodPost, "/api/device-events", map[string]any{
		"device_id":       deviceID,
		"event_key":       "ev-dupe",
		"sequence_number": 2,
		"event_type":      "gate_close",
	}, admin)
	logStep(t, "POST", "/api/device-events", dupe.Code, dupe.Body.String())
	if dupe.Code != http.StatusOK || !strings.Contains(dupe.Body.String(), `"status":"already_processed"`) {
		t.Fatalf("expected duplicate 200, got %d %s", dupe.Code, dupe.Body.String())
	}
}

func TestReplayEventAndDuplicateReplay(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	deviceID, _ := createDeviceWithZone(t, env, admin)

	created := apiRequest(t, env.r, http.MethodPost, "/api/device-events", map[string]any{
		"device_id":       deviceID,
		"event_key":       "ev-replay",
		"sequence_number": 2,
		"event_type":      "camera_ping",
	}, admin)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected 201 ingest, got %d %s", created.Code, created.Body.String())
	}
	logStep(t, "POST", "/api/device-events", created.Code, created.Body.String())

	replay1 := apiRequest(t, env.r, http.MethodPost, "/api/device-events/replay", map[string]any{"event_keys": []string{"ev-replay"}}, admin)
	logStep(t, "POST", "/api/device-events/replay", replay1.Code, replay1.Body.String())
	if replay1.Code != http.StatusOK || !strings.Contains(replay1.Body.String(), `"replayed":1`) {
		t.Fatalf("expected first replay to process event, got %d %s", replay1.Code, replay1.Body.String())
	}

	replay2 := apiRequest(t, env.r, http.MethodPost, "/api/device-events/replay", map[string]any{"event_keys": []string{"ev-replay"}}, admin)
	logStep(t, "POST", "/api/device-events/replay", replay2.Code, replay2.Body.String())
	if replay2.Code != http.StatusOK || !strings.Contains(replay2.Body.String(), `"skipped":1`) {
		t.Fatalf("expected duplicate replay skip, got %d %s", replay2.Code, replay2.Body.String())
	}
}

func TestListDeviceEvents(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	deviceID, _ := createDeviceWithZone(t, env, admin)

	created := apiRequest(t, env.r, http.MethodPost, "/api/device-events", map[string]any{
		"device_id":       deviceID,
		"event_key":       "ev-list",
		"sequence_number": 5,
		"event_type":      "sensor_trigger",
	}, admin)
	logStep(t, "POST", "/api/device-events", created.Code, created.Body.String())

	list := apiRequest(t, env.r, http.MethodGet, "/api/device-events?device_id="+deviceID, nil, admin)
	logStep(t, "GET", "/api/device-events", list.Code, list.Body.String())
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "ev-list") {
		t.Fatalf("expected device event in list, got %d %s", list.Code, list.Body.String())
	}
}

func TestLateEventFlag(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	deviceID, _ := createDeviceWithZone(t, env, admin)

	first := apiRequest(t, env.r, http.MethodPost, "/api/device-events", map[string]any{
		"device_id":       deviceID,
		"event_key":       "ev-late-first",
		"sequence_number": 10,
		"event_type":      "sensor_trigger",
	}, admin)
	logStep(t, "POST", "/api/device-events", first.Code, first.Body.String())
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first event 201, got %d %s", first.Code, first.Body.String())
	}

	_, err := env.pool.Exec(context.Background(), `UPDATE devices SET last_event_received_at = now() - interval '11 minutes' WHERE id = $1`, deviceID)
	if err != nil {
		t.Fatalf("update device last_event_received_at: %v", err)
	}

	late := apiRequest(t, env.r, http.MethodPost, "/api/device-events", map[string]any{
		"device_id":       deviceID,
		"event_key":       "ev-late-second",
		"sequence_number": 9,
		"event_type":      "sensor_trigger",
	}, admin)
	logStep(t, "POST", "/api/device-events", late.Code, late.Body.String())
	if late.Code != http.StatusCreated || !strings.Contains(late.Body.String(), `"late":true`) {
		t.Fatalf("expected late=true on stale out-of-order event, got %d %s", late.Code, late.Body.String())
	}
}

func TestDeviceEventInvalidSignatureNotTrusted(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	deviceID, _ := createDeviceWithZone(t, env, admin)

	deviceTime := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	created := apiRequest(t, env.r, http.MethodPost, "/api/device-events", map[string]any{
		"device_id":             deviceID,
		"event_key":             "ev-invalid-signature",
		"sequence_number":       11,
		"event_type":            "camera_ping",
		"device_time":           deviceTime,
		"device_time_signature": "invalid-signature",
	}, admin)
	logStep(t, "POST", "/api/device-events", created.Code, created.Body.String())
	if created.Code != http.StatusCreated {
		t.Fatalf("expected device event create 201, got %d %s", created.Code, created.Body.String())
	}

	list := apiRequest(t, env.r, http.MethodGet, "/api/device-events?device_id="+deviceID, nil, admin)
	logStep(t, "GET", "/api/device-events", list.Code, list.Body.String())
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "ev-invalid-signature") || !strings.Contains(list.Body.String(), `"device_time_trusted":false`) {
		t.Fatalf("expected event listed as untrusted, got %d %s", list.Code, list.Body.String())
	}
}
