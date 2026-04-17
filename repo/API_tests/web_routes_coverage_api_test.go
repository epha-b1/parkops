package API_tests

// Direct HTTP coverage for the 19 previously uncovered non-/api/* routes:
//
//   POST /auth/login              (form-encoded session login)
//   POST /auth/logout             (form-encoded session logout)
//   GET  /dashboard
//   GET  /capacity
//   GET  /facilities
//   GET  /lots
//   GET  /zones
//   GET  /rate-plans
//   GET  /members
//   GET  /vehicles
//   GET  /drivers
//   GET  /notifications
//   GET  /campaigns
//   GET  /segments
//   GET  /tasks
//   GET  /notification-prefs
//   GET  /analytics
//   GET  /audit
//   GET  /admin/users
//
// All tests drive the real router via httptest.ServeHTTP — no mocks.

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// pageRequest issues an unauthenticated or cookie-authenticated GET against
// a page route and returns the recorder. Follows the same real-router pattern
// used by apiRequest.
func pageRequest(t *testing.T, r http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// formPost issues a form-encoded POST via the real router.
func formPost(t *testing.T, r http.Handler, path string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// sessionCookieFromRecorder extracts the session_id cookie from a response.
func sessionCookieFromRecorder(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == "session_id" {
			return c
		}
	}
	t.Fatalf("session_id cookie not set in response: %v", w.Result().Cookies())
	return nil
}

// ─────────────────────────────────────────────────────────────────────────
// Form login / logout
// ─────────────────────────────────────────────────────────────────────────

func TestFormLoginRedirectsAndSetsSessionCookie(t *testing.T) {
	env := setupAuthAPIEnv(t)

	form := url.Values{}
	form.Set("username", "admin")
	form.Set("password", "AdminPass1234")

	resp := formPost(t, env.r, "/auth/login", form, nil)
	logStep(t, "POST", "/auth/login", resp.Code, "")
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 after form login, got %d body=%s", resp.Code, resp.Body.String())
	}
	loc := resp.Result().Header.Get("Location")
	if !strings.HasPrefix(loc, "/dashboard") {
		t.Fatalf("expected redirect to /dashboard, got %q", loc)
	}
	// Session cookie must be issued
	cookie := sessionCookieFromRecorder(t, resp)
	if cookie.Value == "" {
		t.Fatal("session cookie value is empty after login")
	}
	if !cookie.HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}
}

func TestFormLoginBadCredentialsRedirectsWithError(t *testing.T) {
	env := setupAuthAPIEnv(t)

	form := url.Values{}
	form.Set("username", "admin")
	form.Set("password", "WRONG")

	resp := formPost(t, env.r, "/auth/login", form, nil)
	logStep(t, "POST", "/auth/login (bad creds)", resp.Code, "")
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 after bad login, got %d", resp.Code)
	}
	loc := resp.Result().Header.Get("Location")
	if !strings.Contains(loc, "/login") || !strings.Contains(loc, "login_error") {
		t.Fatalf("expected redirect to /login?toast=login_error, got %q", loc)
	}
}

func TestFormLogoutClearsSessionAndRedirectsToLogin(t *testing.T) {
	env := setupAuthAPIEnv(t)

	// First login via form flow
	form := url.Values{}
	form.Set("username", "admin")
	form.Set("password", "AdminPass1234")
	login := formPost(t, env.r, "/auth/login", form, nil)
	if login.Code != http.StatusSeeOther {
		t.Fatalf("precondition: form login failed: %d", login.Code)
	}
	cookie := sessionCookieFromRecorder(t, login)

	// Now logout
	logout := formPost(t, env.r, "/auth/logout", url.Values{}, cookie)
	logStep(t, "POST", "/auth/logout", logout.Code, "")
	if logout.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 after logout, got %d", logout.Code)
	}
	loc := logout.Result().Header.Get("Location")
	if !strings.HasPrefix(loc, "/login") {
		t.Fatalf("expected redirect to /login after logout, got %q", loc)
	}

	// The logout response must also clear the cookie (MaxAge<0 or empty value).
	cleared := false
	for _, c := range logout.Result().Cookies() {
		if c.Name == "session_id" && (c.MaxAge < 0 || c.Value == "") {
			cleared = true
			break
		}
	}
	if !cleared {
		t.Fatalf("expected session_id cookie to be cleared, got %v", logout.Result().Cookies())
	}

	// Verify session is actually invalidated: subsequent page request with old cookie redirects to /login
	after := pageRequest(t, env.r, "/dashboard", cookie)
	if after.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 for dashboard with invalidated cookie, got %d", after.Code)
	}
	if !strings.HasPrefix(after.Result().Header.Get("Location"), "/login") {
		t.Fatalf("expected redirect to /login after logout, got %q", after.Result().Header.Get("Location"))
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Page route coverage — authenticated, expect 200 + text/html + marker
// ─────────────────────────────────────────────────────────────────────────

// assertPageOK asserts the recorder returned HTTP 200, text/html content type,
// and the response body contains the supplied marker text.
func assertPageOK(t *testing.T, resp *httptest.ResponseRecorder, path, marker string) {
	t.Helper()
	if resp.Code != http.StatusOK {
		t.Fatalf("%s: expected 200, got %d body=%q", path, resp.Code, resp.Body.String())
	}
	ct := resp.Result().Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("%s: expected Content-Type text/html, got %q", path, ct)
	}
	if !strings.Contains(resp.Body.String(), marker) {
		t.Fatalf("%s: body missing marker %q; got: %s", path, marker, resp.Body.String())
	}
}

func TestPageRoutesRenderForAuthenticatedAdmin(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	cases := []struct {
		path   string
		marker string
	}{
		{"/dashboard", "Dashboard"},
		{"/capacity", "Capacity"},
		{"/facilities", "Facilities"},
		{"/lots", "Lots"},
		{"/zones", "Zones"},
		{"/rate-plans", "Rate Plans"},
		{"/members", "Members"},
		{"/vehicles", "Vehicles"},
		{"/drivers", "Drivers"},
		{"/notifications", "Notifications"},
		{"/campaigns", "Campaigns"},
		{"/segments", "Segments"},
		{"/tasks", "Tasks"},
		{"/notification-prefs", "Notification Preferences"},
		{"/analytics", "Analytics"},
	}

	for _, c := range cases {
		resp := pageRequest(t, env.r, c.path, admin)
		logStep(t, "GET", c.path, resp.Code, "")
		assertPageOK(t, resp, c.path, c.marker)
	}
}

func TestAuditPageRendersForAuditor(t *testing.T) {
	env := setupAuthAPIEnv(t)
	auditor := loginAs(t, env, "auditor", "UserPass1234")

	resp := pageRequest(t, env.r, "/audit", auditor)
	logStep(t, "GET", "/audit (auditor)", resp.Code, "")
	assertPageOK(t, resp, "/audit", "Audit Log")
}

func TestAuditPageRendersForAdmin(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	resp := pageRequest(t, env.r, "/audit", admin)
	logStep(t, "GET", "/audit (admin)", resp.Code, "")
	assertPageOK(t, resp, "/audit", "Audit Log")
}

func TestAdminUsersPageRendersForAdmin(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	resp := pageRequest(t, env.r, "/admin/users", admin)
	logStep(t, "GET", "/admin/users (admin)", resp.Code, "")
	assertPageOK(t, resp, "/admin/users", "Admin Users")
}

// ─────────────────────────────────────────────────────────────────────────
// Negative auth case — unauthenticated page request redirects to /login
// ─────────────────────────────────────────────────────────────────────────

func TestPageRoutesRedirectUnauthenticatedToLogin(t *testing.T) {
	env := setupAuthAPIEnv(t)

	// Sample representative protected page routes (role-gated by requireSession).
	for _, path := range []string{"/dashboard", "/capacity", "/analytics", "/audit", "/admin/users"} {
		resp := pageRequest(t, env.r, path, nil)
		logStep(t, "GET", path+" (no auth)", resp.Code, "")
		if resp.Code != http.StatusSeeOther {
			t.Fatalf("%s unauthenticated expected 303, got %d", path, resp.Code)
		}
		loc := resp.Result().Header.Get("Location")
		if !strings.HasPrefix(loc, "/login") {
			t.Fatalf("%s unauthenticated expected redirect to /login, got %q", path, loc)
		}
	}
}
