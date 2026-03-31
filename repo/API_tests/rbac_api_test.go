package API_tests

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func loginAs(t *testing.T, env *apiTestEnv, username, password string) *http.Cookie {
	t.Helper()
	w := apiRequest(t, env.r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": username,
		"password": password,
	}, nil)
	logStep(t, "POST", "/api/auth/login", w.Code, w.Body.String())
	if w.Code != http.StatusOK {
		t.Fatalf("login failed for %s: %d %s", username, w.Code, w.Body.String())
	}
	return loginCookieFromResponse(t, w)
}

func TestDispatchRoleForbiddenEndpointsAndAuditLog(t *testing.T) {
	env := setupAuthAPIEnv(t)
	cookie := loginAs(t, env, "operator", "UserPass1234")

	paths := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/admin/users", nil},
		{http.MethodPost, "/api/admin/users", map[string]any{"username": "x", "password": "ValidPass1234", "roles": []string{"dispatch_operator"}}},
		{http.MethodDelete, "/api/admin/users/22222222-2222-2222-2222-222222222222", nil},
	}

	for _, p := range paths {
		w := apiRequest(t, env.r, p.method, p.path, p.body, cookie)
		logStep(t, p.method, p.path, w.Code, w.Body.String())
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for %s %s, got %d", p.method, p.path, w.Code)
		}
	}

	assertDeniedAuditLogExists(t, env, "/api/admin/users")
}

func TestFleetRoleForbiddenEndpointsAndAuditLog(t *testing.T) {
	env := setupAuthAPIEnv(t)
	cookie := loginAs(t, env, "fleet", "UserPass1234")

	paths := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPatch, "/api/admin/users/22222222-2222-2222-2222-222222222222/roles", map[string]any{"roles": []string{"auditor"}}},
		{http.MethodPost, "/api/admin/users/22222222-2222-2222-2222-222222222222/unlock", nil},
		{http.MethodGet, "/api/admin/audit-logs", nil},
	}

	for _, p := range paths {
		w := apiRequest(t, env.r, p.method, p.path, p.body, cookie)
		logStep(t, p.method, p.path, w.Code, w.Body.String())
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for %s %s, got %d", p.method, p.path, w.Code)
		}
	}

	assertDeniedAuditLogExists(t, env, "/api/admin/audit-logs")
}

func TestAuditorRoleAccessAndForbiddenEndpoints(t *testing.T) {
	env := setupAuthAPIEnv(t)
	cookie := loginAs(t, env, "auditor", "UserPass1234")

	allowed := apiRequest(t, env.r, http.MethodGet, "/api/admin/audit-logs", nil, cookie)
	logStep(t, "GET", "/api/admin/audit-logs", allowed.Code, allowed.Body.String())
	if allowed.Code != http.StatusOK {
		t.Fatalf("auditor should access audit logs, got %d", allowed.Code)
	}

	paths := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/admin/users", nil},
		{http.MethodPatch, "/api/admin/users/22222222-2222-2222-2222-222222222222", map[string]any{"display_name": "blocked"}},
		{http.MethodDelete, "/api/admin/users/22222222-2222-2222-2222-222222222222", nil},
	}

	for _, p := range paths {
		w := apiRequest(t, env.r, p.method, p.path, p.body, cookie)
		logStep(t, p.method, p.path, w.Code, w.Body.String())
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for %s %s, got %d", p.method, p.path, w.Code)
		}
	}

	assertDeniedAuditLogExists(t, env, "/api/admin/users")
}

func TestAdminCanManageUsersAndUpdateRoles(t *testing.T) {
	env := setupAuthAPIEnv(t)
	adminCookie := loginAs(t, env, "admin", "AdminPass1234")

	create := apiRequest(t, env.r, http.MethodPost, "/api/admin/users", map[string]any{
		"username":     "newuser",
		"display_name": "New User",
		"password":     "ValidPass1234",
		"roles":        []string{"dispatch_operator"},
	}, adminCookie)
	logStep(t, "POST", "/api/admin/users", create.Code, create.Body.String())
	if create.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", create.Code, create.Body.String())
	}

	list := apiRequest(t, env.r, http.MethodGet, "/api/admin/users?page=1&limit=10", nil, adminCookie)
	logStep(t, "GET", "/api/admin/users?page=1&limit=10", list.Code, list.Body.String())
	if list.Code != http.StatusOK {
		t.Fatalf("expected list users 200, got %d", list.Code)
	}

	var newUserID string
	err := env.pool.QueryRow(context.Background(), `SELECT id::text FROM users WHERE username='newuser'`).Scan(&newUserID)
	if err != nil {
		t.Fatalf("query new user id: %v", err)
	}

	update := apiRequest(t, env.r, http.MethodPatch, "/api/admin/users/"+newUserID, map[string]any{
		"display_name": "Renamed User",
	}, adminCookie)
	logStep(t, "PATCH", "/api/admin/users/:id", update.Code, update.Body.String())
	if update.Code != http.StatusOK {
		t.Fatalf("expected update user 200, got %d", update.Code)
	}

	roles := apiRequest(t, env.r, http.MethodPatch, "/api/admin/users/"+newUserID+"/roles", map[string]any{
		"roles": []string{"fleet_manager", "auditor"},
	}, adminCookie)
	logStep(t, "PATCH", "/api/admin/users/:id/roles", roles.Code, roles.Body.String())
	if roles.Code != http.StatusOK {
		t.Fatalf("expected update roles 200, got %d", roles.Code)
	}

	del := apiRequest(t, env.r, http.MethodDelete, "/api/admin/users/"+newUserID, nil, adminCookie)
	logStep(t, "DELETE", "/api/admin/users/:id", del.Code, del.Body.String())
	if del.Code != http.StatusNoContent {
		t.Fatalf("expected delete user 204, got %d", del.Code)
	}
}

func assertDeniedAuditLogExists(t *testing.T, env *apiTestEnv, path string) {
	t.Helper()
	var count int
	err := env.pool.QueryRow(context.Background(), `
		SELECT count(*)
		FROM audit_logs
		WHERE action = 'rbac_denied' AND detail->>'path' = $1
	`, path).Scan(&count)
	if err != nil {
		t.Fatalf("query audit log count: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected denied audit log for path %s", path)
	}
}

func TestAdminForbiddenFromAuditorOnlyEndpoint(t *testing.T) {
	env := setupAuthAPIEnv(t)
	adminCookie := loginAs(t, env, "admin", "AdminPass1234")

	w := apiRequest(t, env.r, http.MethodGet, "/api/admin/audit-logs", nil, adminCookie)
	logStep(t, "GET", "/api/admin/audit-logs", w.Code, w.Body.String())
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected admin forbidden on auditor endpoint, got %d", w.Code)
	}

	assertDeniedAuditLogExists(t, env, "/api/admin/audit-logs")
}

func TestRoleEndpointRejectsUnauthorizedRoleUpdate(t *testing.T) {
	env := setupAuthAPIEnv(t)
	opCookie := loginAs(t, env, "operator", "UserPass1234")

	w := apiRequest(t, env.r, http.MethodPatch, "/api/admin/users/33333333-3333-3333-3333-333333333333/roles", map[string]any{
		"roles": []string{"facility_admin"},
	}, opCookie)
	logStep(t, "PATCH", "/api/admin/users/:id/roles", w.Code, w.Body.String())
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden update roles, got %d", w.Code)
	}
}

func TestDeleteUserEndpointForbiddenForDispatch(t *testing.T) {
	env := setupAuthAPIEnv(t)
	opCookie := loginAs(t, env, "operator", "UserPass1234")

	target := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	w := apiRequest(t, env.r, http.MethodDelete, "/api/admin/users/"+target.String(), nil, opCookie)
	logStep(t, "DELETE", "/api/admin/users/:id", w.Code, w.Body.String())
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden delete, got %d", w.Code)
	}
}
