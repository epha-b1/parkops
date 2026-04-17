package API_tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"parkops/internal/auth"
	"parkops/internal/platform/security"
	"parkops/internal/server"
)

type apiTestEnv struct {
	pool *pgxpool.Pool
	r    http.Handler
}

func setupAuthAPIEnv(t *testing.T) *apiTestEnv {
	t.Helper()

	dbURL, hasExplicitDBURL := os.LookupEnv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://parkops:parkops@127.0.0.1:5432/parkops?sslmode=disable"
	}
	// Silent-skip is only allowed when the caller has NOT set TEST_DATABASE_URL
	// AND has opted in via ALLOW_DB_SKIP=1. In any other configuration the
	// suite must fail hard on an unreachable DB so reviewers get a true pass/fail
	// signal rather than an "all skipped" green build.
	_, allowSkip := os.LookupEnv("ALLOW_DB_SKIP")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		if hasExplicitDBURL || !allowSkip {
			t.Fatalf("db unavailable: %v", err)
		}
		t.Skipf("skipping auth api tests, db unavailable: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		if hasExplicitDBURL || !allowSkip {
			t.Fatalf("db unreachable: %v", err)
		}
		t.Skipf("skipping auth api tests, db unreachable: %v", err)
	}

	resetAuthData(t, pool)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := server.NewRouter(logger, pool, []byte("0123456789abcdef0123456789abcdef"))

	t.Cleanup(pool.Close)
	return &apiTestEnv{pool: pool, r: r}
}

func resetAuthData(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		TRUNCATE TABLE
			exports,
			tag_versions,
			segment_runs,
			segment_definitions,
			member_tags,
			tags,
			tasks,
			campaigns,
			notification_jobs,
			notifications,
			notification_subscriptions,
			notification_topics,
			user_dnd_settings,
			compensating_events,
			reconciliation_runs,
			stop_events,
			vehicle_positions,
			exceptions,
			device_events,
			devices,
			booking_events,
			capacity_snapshots,
			capacity_holds,
			reservations,
			message_rules,
			drivers,
			vehicles,
			members,
			rate_plans,
			zones,
			lots,
			facilities,
			sessions,
			user_roles,
			users
		RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("truncate auth tables: %v", err)
	}

	adminHash, err := security.HashPassword("AdminPass1234")
	if err != nil {
		t.Fatalf("hash admin password: %v", err)
	}
	userHash, err := security.HashPassword("UserPass1234")
	if err != nil {
		t.Fatalf("hash user password: %v", err)
	}

	adminID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	fleetID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	auditorID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	_, err = pool.Exec(ctx, `
		INSERT INTO users(id, organization_id, username, password_hash, display_name, status)
		VALUES
		($1, 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid, 'admin', $2, 'Admin', 'active'),
		($3, 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid, 'operator', $4, 'Operator', 'active'),
		($5, 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'::uuid, 'fleet', $4, 'Fleet', 'active'),
		($6, 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'::uuid, 'auditor', $4, 'Auditor', 'active')
	`, adminID, adminHash, userID, userHash, fleetID, auditorID)
	if err != nil {
		t.Fatalf("seed users: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO user_roles(user_id, role_id)
		SELECT $1, id FROM roles WHERE name = 'facility_admin'
	`, adminID)
	if err != nil {
		t.Fatalf("seed admin role: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO user_roles(user_id, role_id)
		SELECT $1, id FROM roles WHERE name = 'dispatch_operator'
	`, userID)
	if err != nil {
		t.Fatalf("seed operator role: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO user_roles(user_id, role_id)
		SELECT $1, id FROM roles WHERE name = 'fleet_manager'
	`, fleetID)
	if err != nil {
		t.Fatalf("seed fleet role: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO user_roles(user_id, role_id)
		SELECT $1, id FROM roles WHERE name = 'auditor'
	`, auditorID)
	if err != nil {
		t.Fatalf("seed auditor role: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO notification_topics(name)
		VALUES ('booking_success'), ('booking_changed'), ('expiry_approaching'), ('arrears_reminder'), ('task_reminder')
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		t.Fatalf("seed notification topics: %v", err)
	}
}

func apiRequest(t *testing.T, r http.Handler, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func logStep(t *testing.T, method, path string, code int, body string) {
	t.Helper()
	t.Logf("  → %s %s", method, path)
	t.Logf("  ← %d %s", code, body)
}

func loginCookieFromResponse(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	res := w.Result()
	for _, c := range res.Cookies() {
		if c.Name == "session_id" {
			return c
		}
	}
	t.Fatal("session cookie not found")
	return nil
}

func extractID(t *testing.T, body string) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("parse id payload: %v", err)
	}
	id, ok := payload["id"].(string)
	if !ok || id == "" {
		t.Fatalf("missing id in payload: %s", body)
	}
	if _, err := uuid.Parse(id); err != nil {
		t.Fatalf("invalid uuid id: %s (%v)", id, err)
	}
	return id
}

func TestLoginSuccess(t *testing.T) {
	env := setupAuthAPIEnv(t)

	w := apiRequest(t, env.r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "AdminPass1234",
	}, nil)
	logStep(t, "POST", "/api/auth/login", w.Code, w.Body.String())

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cookie := loginCookieFromResponse(t, w)
	if !cookie.HttpOnly {
		t.Fatal("expected HttpOnly session cookie")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("expected SameSite=Strict, got %v", cookie.SameSite)
	}
}

func TestLoginWrongPasswordIncrementsCounter(t *testing.T) {
	env := setupAuthAPIEnv(t)

	w := apiRequest(t, env.r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "WrongPass1234",
	}, nil)
	logStep(t, "POST", "/api/auth/login", w.Code, w.Body.String())

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"attempts_remaining":4`) {
		t.Fatalf("expected attempts_remaining in response, got %s", w.Body.String())
	}

	var count int
	err := env.pool.QueryRow(context.Background(), `SELECT failed_login_count FROM users WHERE username = 'admin'`).Scan(&count)
	if err != nil {
		t.Fatalf("query fail counter: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected failed_login_count=1, got %d", count)
	}
}

func TestLoginFailCounterDecrementsCorrectly(t *testing.T) {
	env := setupAuthAPIEnv(t)

	for i := 0; i < 3; i++ {
		w := apiRequest(t, env.r, http.MethodPost, "/api/auth/login", map[string]string{
			"username": "admin",
			"password": "WrongPass1234",
		}, nil)
		logStep(t, "POST", "/api/auth/login", w.Code, w.Body.String())
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d expected 401, got %d", i+1, w.Code)
		}

		var payload map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		rawRemaining, ok := payload["attempts_remaining"]
		if !ok {
			t.Fatalf("attempt %d missing attempts_remaining: %s", i+1, w.Body.String())
		}

		remaining, ok := rawRemaining.(float64)
		if !ok {
			t.Fatalf("attempt %d invalid attempts_remaining type: %T", i+1, rawRemaining)
		}

		expected := float64(auth.LockoutThreshold - (i + 1))
		if remaining != expected {
			t.Fatalf("attempt %d expected attempts_remaining=%v got %v", i+1, expected, remaining)
		}
	}
}

func TestLoginLockoutAfterFiveFails(t *testing.T) {
	env := setupAuthAPIEnv(t)

	for i := 0; i < 4; i++ {
		w := apiRequest(t, env.r, http.MethodPost, "/api/auth/login", map[string]string{
			"username": "admin",
			"password": "WrongPass1234",
		}, nil)
		logStep(t, "POST", "/api/auth/login", w.Code, w.Body.String())
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d expected 401, got %d", i+1, w.Code)
		}
	}

	w := apiRequest(t, env.r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "WrongPass1234",
	}, nil)
	logStep(t, "POST", "/api/auth/login", w.Code, w.Body.String())
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("5th attempt expected 429, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"locked_until":"`) {
		t.Fatalf("expected locked_until in lockout response, got %s", w.Body.String())
	}
}

func TestLogoutAndMe(t *testing.T) {
	env := setupAuthAPIEnv(t)

	login := apiRequest(t, env.r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "AdminPass1234",
	}, nil)
	logStep(t, "POST", "/api/auth/login", login.Code, login.Body.String())
	cookie := loginCookieFromResponse(t, login)

	me := apiRequest(t, env.r, http.MethodGet, "/api/me", nil, cookie)
	logStep(t, "GET", "/api/me", me.Code, me.Body.String())
	if me.Code != http.StatusOK {
		t.Fatalf("expected /api/me 200, got %d", me.Code)
	}

	logout := apiRequest(t, env.r, http.MethodPost, "/api/auth/logout", nil, cookie)
	logStep(t, "POST", "/api/auth/logout", logout.Code, logout.Body.String())
	if logout.Code != http.StatusNoContent {
		t.Fatalf("expected logout 204, got %d", logout.Code)
	}

	meAfter := apiRequest(t, env.r, http.MethodGet, "/api/me", nil, cookie)
	logStep(t, "GET", "/api/me", meAfter.Code, meAfter.Body.String())
	if meAfter.Code != http.StatusUnauthorized {
		t.Fatalf("expected /api/me after logout 401, got %d", meAfter.Code)
	}
}

func TestAdminResetPasswordNoTokenInResponse(t *testing.T) {
	env := setupAuthAPIEnv(t)

	login := apiRequest(t, env.r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "AdminPass1234",
	}, nil)
	logStep(t, "POST", "/api/auth/login", login.Code, login.Body.String())
	adminCookie := loginCookieFromResponse(t, login)

	w := apiRequest(t, env.r, http.MethodPost, "/api/admin/users/22222222-2222-2222-2222-222222222222/reset-password", map[string]string{
		"new_password": "ResetPass1234",
	}, adminCookie)
	logStep(t, "POST", "/api/admin/users/22222222-2222-2222-2222-222222222222/reset-password", w.Code, w.Body.String())

	if w.Code != http.StatusOK {
		t.Fatalf("expected reset 200, got %d: %s", w.Code, w.Body.String())
	}

	body := strings.ToLower(w.Body.String())
	if strings.Contains(body, "token") {
		t.Fatalf("response leaked token field: %s", w.Body.String())
	}

	var force bool
	err := env.pool.QueryRow(context.Background(), `SELECT force_password_change FROM users WHERE id = $1`, uuid.MustParse("22222222-2222-2222-2222-222222222222")).Scan(&force)
	if err != nil {
		t.Fatalf("query force_password_change: %v", err)
	}
	if !force {
		t.Fatal("expected force_password_change=true after admin reset")
	}
}

func TestForcePasswordChangeBlocksRoutesExceptPatch(t *testing.T) {
	env := setupAuthAPIEnv(t)

	_, err := env.pool.Exec(context.Background(), `UPDATE users SET force_password_change = true WHERE username = 'operator'`)
	if err != nil {
		t.Fatalf("set force_password_change: %v", err)
	}

	login := apiRequest(t, env.r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "operator",
		"password": "UserPass1234",
	}, nil)
	logStep(t, "POST", "/api/auth/login", login.Code, login.Body.String())
	if login.Code != http.StatusOK {
		t.Fatalf("operator login failed: %d %s", login.Code, login.Body.String())
	}
	cookie := loginCookieFromResponse(t, login)

	blocked := apiRequest(t, env.r, http.MethodGet, "/api/me", nil, cookie)
	logStep(t, "GET", "/api/me", blocked.Code, blocked.Body.String())
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("expected /api/me blocked with 403, got %d", blocked.Code)
	}

	change := apiRequest(t, env.r, http.MethodPatch, "/api/me/password", map[string]string{
		"current_password": "UserPass1234",
		"new_password":     "UserPass5678",
	}, cookie)
	logStep(t, "PATCH", "/api/me/password", change.Code, change.Body.String())
	if change.Code != http.StatusOK {
		t.Fatalf("expected password change 200, got %d: %s", change.Code, change.Body.String())
	}
}

func TestAdminUnlockAccount(t *testing.T) {
	env := setupAuthAPIEnv(t)

	_, err := env.pool.Exec(context.Background(), `UPDATE users SET failed_login_count = 5, locked_until = now() + interval '15 minutes' WHERE username = 'operator'`)
	if err != nil {
		t.Fatalf("seed locked user: %v", err)
	}

	login := apiRequest(t, env.r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "AdminPass1234",
	}, nil)
	logStep(t, "POST", "/api/auth/login", login.Code, login.Body.String())
	adminCookie := loginCookieFromResponse(t, login)

	w := apiRequest(t, env.r, http.MethodPost, "/api/admin/users/22222222-2222-2222-2222-222222222222/unlock", nil, adminCookie)
	logStep(t, "POST", "/api/admin/users/22222222-2222-2222-2222-222222222222/unlock", w.Code, w.Body.String())
	if w.Code != http.StatusOK {
		t.Fatalf("expected unlock 200, got %d: %s", w.Code, w.Body.String())
	}

	var count int
	var locked *time.Time
	err = env.pool.QueryRow(context.Background(), `SELECT failed_login_count, locked_until FROM users WHERE username = 'operator'`).Scan(&count, &locked)
	if err != nil {
		t.Fatalf("query unlocked user: %v", err)
	}
	if count != 0 || locked != nil {
		t.Fatalf("expected unlocked state, got count=%d locked=%v", count, locked)
	}
}

func TestAdminListAndDeleteUserSessions(t *testing.T) {
	env := setupAuthAPIEnv(t)

	adminLogin := apiRequest(t, env.r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "AdminPass1234",
	}, nil)
	logStep(t, "POST", "/api/auth/login", adminLogin.Code, adminLogin.Body.String())
	adminCookie := loginCookieFromResponse(t, adminLogin)

	userLogin := apiRequest(t, env.r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "operator",
		"password": "UserPass1234",
	}, nil)
	logStep(t, "POST", "/api/auth/login", userLogin.Code, userLogin.Body.String())
	if userLogin.Code != http.StatusOK {
		t.Fatalf("operator login failed: %d %s", userLogin.Code, userLogin.Body.String())
	}

	list := apiRequest(t, env.r, http.MethodGet, "/api/admin/users/22222222-2222-2222-2222-222222222222/sessions", nil, adminCookie)
	logStep(t, "GET", "/api/admin/users/22222222-2222-2222-2222-222222222222/sessions", list.Code, list.Body.String())
	if list.Code != http.StatusOK {
		t.Fatalf("expected list sessions 200, got %d: %s", list.Code, list.Body.String())
	}
	if !strings.Contains(list.Body.String(), "\"id\"") {
		t.Fatalf("expected session payload, got %s", list.Body.String())
	}

	del := apiRequest(t, env.r, http.MethodDelete, "/api/admin/users/22222222-2222-2222-2222-222222222222/sessions", nil, adminCookie)
	logStep(t, "DELETE", "/api/admin/users/22222222-2222-2222-2222-222222222222/sessions", del.Code, del.Body.String())
	if del.Code != http.StatusNoContent {
		t.Fatalf("expected delete sessions 204, got %d: %s", del.Code, del.Body.String())
	}

	var count int
	err := env.pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE user_id = $1`, uuid.MustParse("22222222-2222-2222-2222-222222222222")).Scan(&count)
	if err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 sessions after delete, got %d", count)
	}
}
