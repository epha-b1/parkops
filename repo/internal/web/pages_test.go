package web

// Frontend unit tests for server-rendered Templ pages.
//
// These tests render each page component to a buffer and assert expected
// structure, role-gated navigation, page titles, and presence of the
// interactive elements each page needs to function. They provide direct,
// file-level evidence of frontend module coverage.

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// adminUser / operatorUser / fleetUser / auditorUser test fixtures.
func adminUser() CurrentUser {
	return CurrentUser{DisplayName: "Alice Admin", Username: "admin", Roles: []string{"facility_admin"}}
}

func operatorUser() CurrentUser {
	return CurrentUser{DisplayName: "Dan Dispatch", Username: "operator", Roles: []string{"dispatch_operator"}}
}

func fleetUser() CurrentUser {
	return CurrentUser{DisplayName: "Fran Fleet", Username: "fleet", Roles: []string{"fleet_manager"}}
}

func auditorUser() CurrentUser {
	return CurrentUser{DisplayName: "Pat Auditor", Username: "auditor", Roles: []string{"auditor"}}
}

// ──────────────────────────────────────────────────────────────────────────
// Login page
// ──────────────────────────────────────────────────────────────────────────

func TestLoginPageRendersForm(t *testing.T) {
	var buf bytes.Buffer
	if err := LoginPage().Render(context.Background(), &buf); err != nil {
		t.Fatalf("render login: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `id="loginForm"`) && !strings.Contains(html, `<form`) {
		t.Fatalf("login page missing form element: %s", html)
	}
	if !strings.Contains(html, "username") || !strings.Contains(html, "password") {
		t.Fatalf("login page missing username/password inputs: %s", html)
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Dashboard
// ──────────────────────────────────────────────────────────────────────────

func TestDashboardPageRendersForAllRoles(t *testing.T) {
	for _, user := range []CurrentUser{adminUser(), operatorUser(), fleetUser(), auditorUser()} {
		var buf bytes.Buffer
		if err := DashboardPage(user).Render(context.Background(), &buf); err != nil {
			t.Fatalf("render dashboard for %s: %v", user.Username, err)
		}
		html := buf.String()
		if !strings.Contains(html, "Dashboard") {
			t.Fatalf("dashboard missing title for %s", user.Username)
		}
		// Layout should include user display name
		if !strings.Contains(html, user.DisplayName) {
			t.Fatalf("dashboard missing user display name %q for %s", user.DisplayName, user.Username)
		}
	}
}

func TestDashboardIncludesRoleInfo(t *testing.T) {
	var buf bytes.Buffer
	if err := DashboardPage(adminUser()).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "facility_admin") {
		t.Fatalf("dashboard should surface user role: %s", html)
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Reservations
// ──────────────────────────────────────────────────────────────────────────

func TestReservationsPageRenders(t *testing.T) {
	var buf bytes.Buffer
	if err := ReservationsPage(adminUser()).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "Reservations") {
		t.Fatalf("reservations page missing title")
	}
	// Should include the API base path used by the embedded JS
	if !strings.Contains(html, "/api/reservations") {
		t.Fatalf("reservations page missing API hook: %s", html)
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Capacity
// ──────────────────────────────────────────────────────────────────────────

func TestCapacityPageRenders(t *testing.T) {
	var buf bytes.Buffer
	if err := CapacityPage(adminUser()).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "Capacity") {
		t.Fatalf("capacity page missing title: %s", html)
	}
	if !strings.Contains(html, "/api/capacity/dashboard") {
		t.Fatalf("capacity page missing dashboard endpoint wiring")
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Notifications + Preferences
// ──────────────────────────────────────────────────────────────────────────

func TestNotificationsPageRenders(t *testing.T) {
	var buf bytes.Buffer
	if err := NotificationsPage(adminUser()).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "Notifications") {
		t.Fatalf("notifications page missing title")
	}
	if !strings.Contains(html, "/api/notifications") {
		t.Fatalf("notifications page missing API endpoint")
	}
}

func TestNotificationPrefsPageUsesCorrectBackendRoutes(t *testing.T) {
	var buf bytes.Buffer
	if err := NotificationPrefsPage(adminUser()).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	// Must call the actual backend endpoints (regression guard against the earlier
	// mismatch flagged in the repair verification report).
	requiredRoutes := []string{
		"/api/notification-topics",
		"/api/notification-topics/",          // subscribe/unsubscribe path prefix
		"/api/notification-settings/dnd",
	}
	for _, r := range requiredRoutes {
		if !strings.Contains(html, r) {
			t.Fatalf("notification prefs page missing required route %q", r)
		}
	}
	// Must NOT call the non-existent legacy routes.
	forbiddenRoutes := []string{
		"/api/notifications/subscriptions",
		"/api/notifications/subscribe",
		"/api/notifications/unsubscribe",
		"/api/notifications/dnd",
	}
	for _, r := range forbiddenRoutes {
		if strings.Contains(html, r) {
			t.Fatalf("notification prefs page still references non-existent route %q", r)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Analytics
// ──────────────────────────────────────────────────────────────────────────

func TestAnalyticsPageExposesAllPivotOptions(t *testing.T) {
	var buf bytes.Buffer
	if err := AnalyticsPage(adminUser()).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "Analytics") {
		t.Fatalf("analytics missing title")
	}
	pivots := []string{`value="time"`, `value="region"`, `value="category"`, `value="revenue"`}
	for _, p := range pivots {
		if !strings.Contains(html, p) {
			t.Fatalf("analytics pivot option missing: %s", p)
		}
	}
	// Must include export UI hooks
	if !strings.Contains(html, "/api/exports") {
		t.Fatalf("analytics page missing exports endpoint hook")
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Tasks
// ──────────────────────────────────────────────────────────────────────────

func TestTasksPageRendersFormAndActions(t *testing.T) {
	var buf bytes.Buffer
	if err := TasksPage(adminUser()).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "Tasks") {
		t.Fatalf("tasks page missing title")
	}
	// Must have the create dialog and the completion action wiring
	if !strings.Contains(html, "taskDialog") {
		t.Fatalf("tasks page missing create dialog element")
	}
	if !strings.Contains(html, "data-complete") {
		t.Fatalf("tasks page missing completion handler selector")
	}
	// Must hit the real backend endpoint shape
	if !strings.Contains(html, "/api/campaigns") || !strings.Contains(html, "/api/tasks/") {
		t.Fatalf("tasks page missing API endpoints: %s", html)
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Role-gated navigation (layout + buildNav)
// ──────────────────────────────────────────────────────────────────────────

func TestNavigationGatesAdminOnlyLinksByRole(t *testing.T) {
	// Admin sees Facilities / Lots / Zones / Rate Plans / Admin Users
	var adminBuf bytes.Buffer
	if err := DashboardPage(adminUser()).Render(context.Background(), &adminBuf); err != nil {
		t.Fatalf("render admin: %v", err)
	}
	adminHTML := adminBuf.String()
	for _, path := range []string{"/facilities", "/lots", "/zones", "/rate-plans", "/admin/users"} {
		if !strings.Contains(adminHTML, `href="`+path+`"`) {
			t.Fatalf("admin nav should include %s", path)
		}
	}

	// Dispatch operator should NOT see facility/lot/zone/rate-plan admin links
	var opBuf bytes.Buffer
	if err := DashboardPage(operatorUser()).Render(context.Background(), &opBuf); err != nil {
		t.Fatalf("render operator: %v", err)
	}
	opHTML := opBuf.String()
	for _, path := range []string{"/facilities", "/lots", "/zones", "/rate-plans", "/admin/users"} {
		if strings.Contains(opHTML, `href="`+path+`"`) {
			t.Fatalf("operator nav should NOT include %s", path)
		}
	}

	// Auditor should see Audit Log link
	var audBuf bytes.Buffer
	if err := DashboardPage(auditorUser()).Render(context.Background(), &audBuf); err != nil {
		t.Fatalf("render auditor: %v", err)
	}
	if !strings.Contains(audBuf.String(), `href="/audit"`) {
		t.Fatalf("auditor should see audit log link")
	}

	// Fleet manager should see Vehicles / Drivers / Members
	var flBuf bytes.Buffer
	if err := DashboardPage(fleetUser()).Render(context.Background(), &flBuf); err != nil {
		t.Fatalf("render fleet: %v", err)
	}
	flHTML := flBuf.String()
	for _, path := range []string{"/members", "/vehicles", "/drivers"} {
		if !strings.Contains(flHTML, `href="`+path+`"`) {
			t.Fatalf("fleet nav should include %s", path)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Layout helpers (pure functions) — exhaustive unit coverage
// ──────────────────────────────────────────────────────────────────────────

func TestInitialsForHandlesEdgeCases(t *testing.T) {
	cases := map[string]string{
		"Alice Admin":     "AA",
		"alice":           "AL",
		"a":               "A",
		"":                "",
		"Mary Anne Smith": "MS", // first + last
	}
	for in, want := range cases {
		got := initialsFor(in)
		if got != want {
			t.Errorf("initialsFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsNavVisibleEmptyAllowedIsPublic(t *testing.T) {
	if !isNavVisible([]string{"any_role"}, nil) {
		t.Fatal("empty allowed roles should be visible to everyone")
	}
	if !isNavVisible(nil, nil) {
		t.Fatal("empty allowed roles should be visible even with no user roles")
	}
}

func TestIsNavVisibleMatchingRole(t *testing.T) {
	if !isNavVisible([]string{"dispatch_operator"}, []string{"facility_admin", "dispatch_operator"}) {
		t.Fatal("dispatch_operator should match allowed list")
	}
	if isNavVisible([]string{"auditor"}, []string{"facility_admin"}) {
		t.Fatal("auditor should not match facility_admin-only list")
	}
}

// ──────────────────────────────────────────────────────────────────────────
// CRUD page factory
// ──────────────────────────────────────────────────────────────────────────

func TestCrudPageRendersFieldsAndEndpoint(t *testing.T) {
	cfg := CrudPageConfig{
		Title:     "Facilities",
		Path:      "/facilities",
		APIBase:   "/api/facilities",
		CanCreate: true,
		CanEdit:   true,
		CanDelete: true,
		Fields: []CrudField{
			{Key: "name", Label: "Name", Type: "text", Required: true, Placeholder: "Downtown"},
			{Key: "address", Label: "Address", Type: "text"},
		},
	}
	var buf bytes.Buffer
	if err := CrudPage(adminUser(), cfg).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render crud page: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "Facilities") {
		t.Fatalf("crud page missing title")
	}
	if !strings.Contains(html, "/api/facilities") {
		t.Fatalf("crud page missing APIBase: %s", html)
	}
	// Field labels should be rendered
	for _, label := range []string{"Name", "Address"} {
		if !strings.Contains(html, label) {
			t.Fatalf("crud page missing field label %q", label)
		}
	}
}
