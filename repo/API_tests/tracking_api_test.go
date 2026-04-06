package API_tests

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
	"time"
)

func signDeviceTime(deviceTime, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(deviceTime))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestSubmitTrackingLocationAndGetPositions(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fx := createReservationFixture(t, env, admin, 10, 15)

	if fx.vehicleSigningSecret == "" {
		t.Fatal("vehicle signing secret not available in fixture")
	}
	deviceTime := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	sig := signDeviceTime(deviceTime, fx.vehicleSigningSecret)

	create := apiRequest(t, env.r, http.MethodPost, "/api/tracking/location", map[string]any{
		"vehicle_id":            fx.vehicleID,
		"latitude":              37.7749,
		"longitude":             -122.4194,
		"device_time":           deviceTime,
		"device_time_signature": sig,
	}, admin)
	logStep(t, "POST", "/api/tracking/location", create.Code, create.Body.String())
	if create.Code != http.StatusCreated || !strings.Contains(create.Body.String(), `"device_time_trusted":true`) {
		t.Fatalf("expected tracking location create with trusted timestamp, got %d %s", create.Code, create.Body.String())
	}

	positions := apiRequest(t, env.r, http.MethodGet, "/api/tracking/vehicles/"+fx.vehicleID+"/positions", nil, admin)
	logStep(t, "GET", "/api/tracking/vehicles/:id/positions", positions.Code, positions.Body.String())
	if positions.Code != http.StatusOK || !strings.Contains(positions.Body.String(), `"device_time_trusted":true`) {
		t.Fatalf("expected positions list with trusted timestamp, got %d %s", positions.Code, positions.Body.String())
	}
}

func TestTrackingInvalidSignatureNotTrusted(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fx := createReservationFixture(t, env, admin, 10, 15)

	deviceTime := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	create := apiRequest(t, env.r, http.MethodPost, "/api/tracking/location", map[string]any{
		"vehicle_id":            fx.vehicleID,
		"latitude":              37.7749,
		"longitude":             -122.4194,
		"device_time":           deviceTime,
		"device_time_signature": "invalid-signature",
	}, admin)
	logStep(t, "POST", "/api/tracking/location", create.Code, create.Body.String())
	if create.Code != http.StatusCreated || !strings.Contains(create.Body.String(), `"device_time_trusted":false`) {
		t.Fatalf("expected tracking location create with untrusted timestamp, got %d %s", create.Code, create.Body.String())
	}
}

func TestTrackingPlateNumberAsSecretNotTrusted(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fx := createReservationFixture(t, env, admin, 10, 15)

	// Sign using plate number (old, insecure approach) -- should NOT be trusted
	deviceTime := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	sig := signDeviceTime(deviceTime, "RES-123") // plate number, not dedicated secret

	create := apiRequest(t, env.r, http.MethodPost, "/api/tracking/location", map[string]any{
		"vehicle_id":            fx.vehicleID,
		"latitude":              37.7749,
		"longitude":             -122.4194,
		"device_time":           deviceTime,
		"device_time_signature": sig,
	}, admin)
	logStep(t, "POST", "/api/tracking/location (plate secret)", create.Code, create.Body.String())
	if create.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d %s", create.Code, create.Body.String())
	}
	if strings.Contains(create.Body.String(), `"device_time_trusted":true`) {
		t.Fatalf("plate number as secret should NOT produce trusted timestamp")
	}
}

func TestTrackingDedicatedSecretProducesTrusted(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fx := createReservationFixture(t, env, admin, 10, 15)

	if fx.vehicleSigningSecret == "" {
		t.Fatal("vehicle signing secret not available")
	}
	deviceTime := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	sig := signDeviceTime(deviceTime, fx.vehicleSigningSecret)

	create := apiRequest(t, env.r, http.MethodPost, "/api/tracking/location", map[string]any{
		"vehicle_id":            fx.vehicleID,
		"latitude":              37.7749,
		"longitude":             -122.4194,
		"device_time":           deviceTime,
		"device_time_signature": sig,
	}, admin)
	logStep(t, "POST", "/api/tracking/location (dedicated secret)", create.Code, create.Body.String())
	if create.Code != http.StatusCreated || !strings.Contains(create.Body.String(), `"device_time_trusted":true`) {
		t.Fatalf("dedicated secret should produce trusted timestamp, got %d %s", create.Code, create.Body.String())
	}
}

func TestTrackingSigningSecretNotExposedInVehicleAPI(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fx := createReservationFixture(t, env, admin, 10, 15)

	resp := apiRequest(t, env.r, http.MethodGet, "/api/vehicles/"+fx.vehicleID, nil, admin)
	logStep(t, "GET", "/api/vehicles/:id", resp.Code, resp.Body.String())
	if resp.Code != http.StatusOK {
		t.Fatalf("get vehicle failed: %d %s", resp.Code, resp.Body.String())
	}
	if strings.Contains(resp.Body.String(), "signing_secret") {
		t.Fatalf("signing secret should NOT be exposed in vehicle API response")
	}
}

func TestTrackingSuspectPositionHandling(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fx := createReservationFixture(t, env, admin, 10, 15)

	first := apiRequest(t, env.r, http.MethodPost, "/api/tracking/location", map[string]any{
		"vehicle_id": fx.vehicleID,
		"latitude":   37.7749,
		"longitude":  -122.4194,
	}, admin)
	logStep(t, "POST", "/api/tracking/location", first.Code, first.Body.String())
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first location 201, got %d %s", first.Code, first.Body.String())
	}

	suspect := apiRequest(t, env.r, http.MethodPost, "/api/tracking/location", map[string]any{
		"vehicle_id": fx.vehicleID,
		"latitude":   37.7810,
		"longitude":  -122.4194,
	}, admin)
	logStep(t, "POST", "/api/tracking/location", suspect.Code, suspect.Body.String())
	if suspect.Code != http.StatusCreated || !strings.Contains(suspect.Body.String(), `"suspect":true`) {
		t.Fatalf("expected suspect held report, got %d %s", suspect.Code, suspect.Body.String())
	}
	suspectID := extractID(t, suspect.Body.String())

	confirmOrDiscard := apiRequest(t, env.r, http.MethodPost, "/api/tracking/location", map[string]any{
		"vehicle_id": fx.vehicleID,
		"latitude":   37.77495,
		"longitude":  -122.41945,
	}, admin)
	logStep(t, "POST", "/api/tracking/location", confirmOrDiscard.Code, confirmOrDiscard.Body.String())
	if confirmOrDiscard.Code != http.StatusCreated {
		t.Fatalf("expected follow-up report 201, got %d %s", confirmOrDiscard.Code, confirmOrDiscard.Body.String())
	}

	suspects := apiRequest(t, env.r, http.MethodGet, "/api/tracking/vehicles/"+fx.vehicleID+"/positions?suspect=true", nil, admin)
	logStep(t, "GET", "/api/tracking/vehicles/:id/positions?suspect=true", suspects.Code, suspects.Body.String())
	if suspects.Code != http.StatusOK || strings.Contains(suspects.Body.String(), suspectID) {
		t.Fatalf("expected discarded suspect not returned, got %d %s", suspects.Code, suspects.Body.String())
	}
}

func TestTrackingStopEventsEndpoint(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	fx := createReservationFixture(t, env, admin, 10, 15)

	first := apiRequest(t, env.r, http.MethodPost, "/api/tracking/location", map[string]any{
		"vehicle_id": fx.vehicleID,
		"latitude":   37.7749,
		"longitude":  -122.4194,
	}, admin)
	logStep(t, "POST", "/api/tracking/location", first.Code, first.Body.String())
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first location 201, got %d %s", first.Code, first.Body.String())
	}
	firstID := extractID(t, first.Body.String())

	_, err := env.pool.Exec(context.Background(), `UPDATE vehicle_positions SET received_at = now() - interval '3 minutes' WHERE id = $1::uuid`, firstID)
	if err != nil {
		t.Fatalf("age first position: %v", err)
	}

	second := apiRequest(t, env.r, http.MethodPost, "/api/tracking/location", map[string]any{
		"vehicle_id": fx.vehicleID,
		"latitude":   37.77495,
		"longitude":  -122.41945,
	}, admin)
	logStep(t, "POST", "/api/tracking/location", second.Code, second.Body.String())
	if second.Code != http.StatusCreated {
		t.Fatalf("expected second location 201, got %d %s", second.Code, second.Body.String())
	}

	stops := apiRequest(t, env.r, http.MethodGet, "/api/tracking/vehicles/"+fx.vehicleID+"/stops", nil, admin)
	logStep(t, "GET", "/api/tracking/vehicles/:id/stops", stops.Code, stops.Body.String())
	if stops.Code != http.StatusOK || !strings.Contains(stops.Body.String(), `"started_at"`) {
		t.Fatalf("expected stop event in response, got %d %s", stops.Code, stops.Body.String())
	}
}
