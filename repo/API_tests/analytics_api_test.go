package API_tests

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAnalyticsOccupancyEndpoint(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	from := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)

	resp := apiRequest(t, env.r, http.MethodGet, "/api/analytics/occupancy?from="+from+"&to="+to, nil, admin)
	logStep(t, "GET", "/api/analytics/occupancy", resp.Code, resp.Body.String())
	if resp.Code != http.StatusOK {
		t.Fatalf("analytics occupancy failed: %d %s", resp.Code, resp.Body.String())
	}
}

func TestAnalyticsBookingsEndpoint(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	resp := apiRequest(t, env.r, http.MethodGet, "/api/analytics/bookings?pivot_by=time", nil, admin)
	logStep(t, "GET", "/api/analytics/bookings", resp.Code, resp.Body.String())
	if resp.Code != http.StatusOK {
		t.Fatalf("analytics bookings failed: %d %s", resp.Code, resp.Body.String())
	}
}

func TestAnalyticsExceptionsEndpoint(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	resp := apiRequest(t, env.r, http.MethodGet, "/api/analytics/exceptions", nil, admin)
	logStep(t, "GET", "/api/analytics/exceptions", resp.Code, resp.Body.String())
	if resp.Code != http.StatusOK {
		t.Fatalf("analytics exceptions failed: %d %s", resp.Code, resp.Body.String())
	}
}

func TestExportCRUDAndDownload(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	// Create CSV export
	createExport := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format": "csv",
		"scope":  "bookings",
	}, admin)
	logStep(t, "POST", "/api/exports", createExport.Code, createExport.Body.String())
	if createExport.Code != http.StatusCreated {
		t.Fatalf("create export failed: %d %s", createExport.Code, createExport.Body.String())
	}
	exportID := extractID(t, createExport.Body.String())
	if !strings.Contains(createExport.Body.String(), `"status":"ready"`) {
		t.Fatalf("expected export status ready: %s", createExport.Body.String())
	}

	// List exports
	listExports := apiRequest(t, env.r, http.MethodGet, "/api/exports", nil, admin)
	logStep(t, "GET", "/api/exports", listExports.Code, listExports.Body.String())
	if listExports.Code != http.StatusOK || !strings.Contains(listExports.Body.String(), exportID) {
		t.Fatalf("list exports failed: %d %s", listExports.Code, listExports.Body.String())
	}

	// Download export
	download := apiRequest(t, env.r, http.MethodGet, "/api/exports/"+exportID+"/download", nil, admin)
	logStep(t, "GET", "/api/exports/:id/download", download.Code, download.Body.String())
	if download.Code != http.StatusOK {
		t.Fatalf("download export failed: %d %s", download.Code, download.Body.String())
	}
	if !strings.Contains(download.Body.String(), "id,zone_id,status,stall_count,created_at") {
		t.Fatalf("expected CSV headers in download, got: %s", download.Body.String())
	}
}

func TestExportRoleForbidden(t *testing.T) {
	env := setupAuthAPIEnv(t)
	auditor := loginAs(t, env, "auditor", "UserPass1234")

	// Auditor should be forbidden from creating exports
	createExport := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format": "csv",
		"scope":  "bookings",
	}, auditor)
	logStep(t, "POST", "/api/exports (auditor)", createExport.Code, createExport.Body.String())
	if createExport.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for auditor creating export, got %d", createExport.Code)
	}
}

func TestExportOwnershipRestrictedForAuditor(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	auditor := loginAs(t, env, "auditor", "UserPass1234")

	createExport := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format": "csv",
		"scope":  "bookings",
	}, admin)
	logStep(t, "POST", "/api/exports (admin)", createExport.Code, createExport.Body.String())
	if createExport.Code != http.StatusCreated {
		t.Fatalf("create export failed: %d %s", createExport.Code, createExport.Body.String())
	}
	exportID := extractID(t, createExport.Body.String())

	listExports := apiRequest(t, env.r, http.MethodGet, "/api/exports", nil, auditor)
	logStep(t, "GET", "/api/exports (auditor)", listExports.Code, listExports.Body.String())
	if listExports.Code != http.StatusOK {
		t.Fatalf("list exports failed: %d %s", listExports.Code, listExports.Body.String())
	}
	if strings.Contains(listExports.Body.String(), exportID) {
		t.Fatalf("auditor should not see other users' exports")
	}

	download := apiRequest(t, env.r, http.MethodGet, "/api/exports/"+exportID+"/download", nil, auditor)
	logStep(t, "GET", "/api/exports/:id/download (auditor)", download.Code, download.Body.String())
	if download.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for auditor downloading another user's export, got %d", download.Code)
	}
}

func TestExportSegmentRequiresAdmin(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	operator := loginAs(t, env, "operator", "UserPass1234")

	createSeg := apiRequest(t, env.r, http.MethodPost, "/api/segments", map[string]any{
		"name":              "Segmented Export",
		"filter_expression": map[string]any{"arrears_balance_cents": map[string]any{"gt": 1}},
		"schedule":          "manual",
	}, admin)
	logStep(t, "POST", "/api/segments", createSeg.Code, createSeg.Body.String())
	if createSeg.Code != http.StatusCreated {
		t.Fatalf("create segment failed: %d %s", createSeg.Code, createSeg.Body.String())
	}
	segmentID := extractID(t, createSeg.Body.String())

	createExport := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format":     "csv",
		"scope":      "bookings",
		"segment_id": segmentID,
	}, operator)
	logStep(t, "POST", "/api/exports (segment by operator)", createExport.Code, createExport.Body.String())
	if createExport.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for operator segment export, got %d", createExport.Code)
	}
}

func TestExportOccupancyCSV(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	createExport := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format": "csv",
		"scope":  "occupancy",
	}, admin)
	logStep(t, "POST", "/api/exports (occupancy)", createExport.Code, createExport.Body.String())
	if createExport.Code != http.StatusCreated {
		t.Fatalf("create occupancy export failed: %d %s", createExport.Code, createExport.Body.String())
	}

	exportID := extractID(t, createExport.Body.String())
	download := apiRequest(t, env.r, http.MethodGet, "/api/exports/"+exportID+"/download", nil, admin)
	logStep(t, "GET", "/api/exports/:id/download (occupancy)", download.Code, download.Body.String())
	if download.Code != http.StatusOK {
		t.Fatalf("download failed: %d %s", download.Code, download.Body.String())
	}
}

func TestExportExceptionsCSV(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	createExport := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format": "csv",
		"scope":  "exceptions",
	}, admin)
	logStep(t, "POST", "/api/exports (exceptions)", createExport.Code, createExport.Body.String())
	if createExport.Code != http.StatusCreated {
		t.Fatalf("create exceptions export failed: %d %s", createExport.Code, createExport.Body.String())
	}
}

// --- Export format tests ---

func TestExportExcelCreateAndDownload(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	create := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format": "excel",
		"scope":  "bookings",
	}, admin)
	logStep(t, "POST", "/api/exports (excel)", create.Code, create.Body.String())
	if create.Code != http.StatusCreated {
		t.Fatalf("create excel export failed: %d %s", create.Code, create.Body.String())
	}
	if !strings.Contains(create.Body.String(), `"format":"excel"`) {
		t.Fatalf("expected format excel in response: %s", create.Body.String())
	}
	exportID := extractID(t, create.Body.String())

	download := apiRequest(t, env.r, http.MethodGet, "/api/exports/"+exportID+"/download", nil, admin)
	logStep(t, "GET", "/api/exports/:id/download (excel)", download.Code, download.Body.String())
	if download.Code != http.StatusOK {
		t.Fatalf("download excel export failed: %d %s", download.Code, download.Body.String())
	}
	ct := download.Result().Header.Get("Content-Type")
	if !strings.Contains(ct, "spreadsheetml") {
		t.Fatalf("expected excel content type, got %s", ct)
	}
	disp := download.Result().Header.Get("Content-Disposition")
	if !strings.Contains(disp, "export.xlsx") {
		t.Fatalf("expected export.xlsx filename, got %s", disp)
	}
}

func TestExportPDFCreateAndDownload(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	create := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format": "pdf",
		"scope":  "exceptions",
	}, admin)
	logStep(t, "POST", "/api/exports (pdf)", create.Code, create.Body.String())
	if create.Code != http.StatusCreated {
		t.Fatalf("create pdf export failed: %d %s", create.Code, create.Body.String())
	}
	if !strings.Contains(create.Body.String(), `"format":"pdf"`) {
		t.Fatalf("expected format pdf in response: %s", create.Body.String())
	}
	exportID := extractID(t, create.Body.String())

	download := apiRequest(t, env.r, http.MethodGet, "/api/exports/"+exportID+"/download", nil, admin)
	logStep(t, "GET", "/api/exports/:id/download (pdf)", download.Code, download.Body.String())
	if download.Code != http.StatusOK {
		t.Fatalf("download pdf export failed: %d %s", download.Code, download.Body.String())
	}
	ct := download.Result().Header.Get("Content-Type")
	if !strings.Contains(ct, "pdf") {
		t.Fatalf("expected pdf content type, got %s", ct)
	}
	disp := download.Result().Header.Get("Content-Disposition")
	if !strings.Contains(disp, "export.pdf") {
		t.Fatalf("expected export.pdf filename, got %s", disp)
	}
}

func TestExportInvalidFormatRejected(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	create := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format": "xml",
		"scope":  "bookings",
	}, admin)
	logStep(t, "POST", "/api/exports (xml)", create.Code, create.Body.String())
	if create.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid format, got %d", create.Code)
	}
}

func TestExportCSVBackwardCompatible(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	create := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format": "csv",
		"scope":  "bookings",
	}, admin)
	logStep(t, "POST", "/api/exports (csv compat)", create.Code, create.Body.String())
	if create.Code != http.StatusCreated {
		t.Fatalf("csv compat failed: %d %s", create.Code, create.Body.String())
	}

	exportID := extractID(t, create.Body.String())
	download := apiRequest(t, env.r, http.MethodGet, "/api/exports/"+exportID+"/download", nil, admin)
	if download.Code != http.StatusOK {
		t.Fatalf("csv download failed: %d %s", download.Code, download.Body.String())
	}
	ct := download.Result().Header.Get("Content-Type")
	if !strings.Contains(ct, "text/csv") {
		t.Fatalf("expected text/csv, got %s", ct)
	}
}

// --- Export segment authorization tests ---

func TestExportSegmentMembershipDeniedForOutOfScopeOperator(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	// Create segment as admin
	createSeg := apiRequest(t, env.r, http.MethodPost, "/api/segments", map[string]any{
		"name":              "Auth Test Segment",
		"filter_expression": map[string]any{"arrears_balance_cents": map[string]any{"gt": 1}},
		"schedule":          "manual",
	}, admin)
	if createSeg.Code != http.StatusCreated {
		t.Fatalf("create segment: %d %s", createSeg.Code, createSeg.Body.String())
	}
	segmentID := extractID(t, createSeg.Body.String())

	// Operator (dispatch) should be denied segment export since they need segment membership
	operator := loginAs(t, env, "operator", "UserPass1234")
	createExport := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format":     "csv",
		"scope":      "bookings",
		"segment_id": segmentID,
	}, operator)
	logStep(t, "POST", "/api/exports (segment by operator)", createExport.Code, createExport.Body.String())
	if createExport.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for operator segment export without membership, got %d", createExport.Code)
	}
}

func TestExportSegmentAllowedForAdmin(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	createSeg := apiRequest(t, env.r, http.MethodPost, "/api/segments", map[string]any{
		"name":              "Admin Segment Test",
		"filter_expression": map[string]any{"arrears_balance_cents": map[string]any{"gt": 1}},
		"schedule":          "manual",
	}, admin)
	if createSeg.Code != http.StatusCreated {
		t.Fatalf("create segment: %d %s", createSeg.Code, createSeg.Body.String())
	}
	segmentID := extractID(t, createSeg.Body.String())

	createExport := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format":     "csv",
		"scope":      "bookings",
		"segment_id": segmentID,
	}, admin)
	logStep(t, "POST", "/api/exports (segment by admin)", createExport.Code, createExport.Body.String())
	if createExport.Code != http.StatusCreated {
		t.Fatalf("expected admin segment export to succeed, got %d %s", createExport.Code, createExport.Body.String())
	}
}

func TestAnalyticsOccupancyMissingParams(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	resp := apiRequest(t, env.r, http.MethodGet, "/api/analytics/occupancy", nil, admin)
	logStep(t, "GET", "/api/analytics/occupancy (no params)", resp.Code, resp.Body.String())
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing from/to, got %d", resp.Code)
	}
}
