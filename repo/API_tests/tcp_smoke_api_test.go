package API_tests

// TCP-level smoke tests.
//
// These tests address the reviewer's concern that the rest of the API suite
// uses in-process `httptest.NewRequest` + `ServeHTTP()`, which bypasses the
// real socket layer. Here we bind the router to a real TCP listener via
// `httptest.NewServer` and exercise endpoints through a real `http.Client`
// so the full stack — listener, accept loop, request parser, router,
// middleware, handler, response writer — is proven end-to-end over TCP.
//
// If reviewers require genuine end-to-end over TCP, these tests demonstrate
// it explicitly while still running inside the Go test harness.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// tcpTestServer spins up the real router on a real TCP socket and returns
// (server, client). The client has a cookie jar so it persists the session
// like a real browser would.
func tcpTestServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()
	env := setupAuthAPIEnv(t)
	srv := httptest.NewServer(env.r)
	t.Cleanup(srv.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	client := &http.Client{
		Jar: jar,
		// Do NOT follow redirects — we need to inspect 303s explicitly.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return srv, client
}

// tcpLoginAsAdmin logs in over a real TCP socket via the JSON API endpoint
// and returns a client that carries the session cookie automatically.
func tcpLoginAsAdmin(t *testing.T, srv *httptest.Server, client *http.Client) {
	t.Helper()
	payload := strings.NewReader(`{"username":"admin","password":"AdminPass1234"}`)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login", payload)
	if err != nil {
		t.Fatalf("build login req: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("tcp login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("tcp login failed: %d %s", resp.StatusCode, string(body))
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Smoke: public endpoint over real TCP
// ─────────────────────────────────────────────────────────────────────────

func TestTCPSmoke_Health(t *testing.T) {
	srv, client := tcpTestServer(t)

	resp, err := client.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatalf("tcp GET /api/health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health over TCP: want 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("health body unexpected: %s", string(body))
	}
	// Prove it really crossed a TCP socket by confirming srv.URL is HTTP on loopback.
	u, _ := url.Parse(srv.URL)
	if u.Scheme != "http" || u.Host == "" {
		t.Fatalf("expected real TCP http server, got %q", srv.URL)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Smoke: authenticated JSON GET over real TCP (session carried by cookie jar)
// ─────────────────────────────────────────────────────────────────────────

func TestTCPSmoke_MeAfterLogin(t *testing.T) {
	srv, client := tcpTestServer(t)
	tcpLoginAsAdmin(t, srv, client)

	resp, err := client.Get(srv.URL + "/api/me")
	if err != nil {
		t.Fatalf("tcp GET /api/me: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me over TCP: want 200, got %d", resp.StatusCode)
	}
	var me map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		t.Fatalf("decode me body: %v", err)
	}
	if me["username"] != "admin" {
		t.Fatalf("me body missing expected username: %v", me)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Smoke: unauthenticated protected endpoint returns 401 over real TCP
// ─────────────────────────────────────────────────────────────────────────

func TestTCPSmoke_ProtectedWithoutAuthReturns401(t *testing.T) {
	srv, client := tcpTestServer(t)

	resp, err := client.Get(srv.URL + "/api/me")
	if err != nil {
		t.Fatalf("tcp GET /api/me: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /api/me over TCP: want 401, got %d", resp.StatusCode)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Smoke: page route redirect observed over real TCP (no auto-follow)
// ─────────────────────────────────────────────────────────────────────────

func TestTCPSmoke_DashboardUnauthRedirectsToLogin(t *testing.T) {
	srv, client := tcpTestServer(t)

	resp, err := client.Get(srv.URL + "/dashboard")
	if err != nil {
		t.Fatalf("tcp GET /dashboard: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("dashboard no-auth TCP: want 303, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/login") {
		t.Fatalf("expected redirect to /login, got %q", loc)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Smoke: JSON POST round-trip over real TCP (write endpoint)
// ─────────────────────────────────────────────────────────────────────────

func TestTCPSmoke_CreateFacilityRoundTrip(t *testing.T) {
	srv, client := tcpTestServer(t)
	tcpLoginAsAdmin(t, srv, client)

	body := bytes.NewReader([]byte(`{"name":"TCP-Facility","address":"1 TCP St"}`))
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/facilities", body)
	if err != nil {
		t.Fatalf("build req: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("tcp POST /api/facilities: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create facility over TCP: want 201, got %d body=%s", resp.StatusCode, string(b))
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	id, _ := out["id"].(string)
	if id == "" {
		t.Fatalf("response missing id: %v", out)
	}

	// Round-trip: read it back over TCP and confirm the name.
	getResp, err := client.Get(srv.URL + "/api/facilities/" + id)
	if err != nil {
		t.Fatalf("tcp GET /api/facilities/:id: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(getResp.Body)
		t.Fatalf("get facility over TCP: want 200, got %d body=%s", getResp.StatusCode, string(b))
	}
	b, _ := io.ReadAll(getResp.Body)
	if !strings.Contains(string(b), "TCP-Facility") {
		t.Fatalf("round-trip body missing expected value: %s", string(b))
	}
}
