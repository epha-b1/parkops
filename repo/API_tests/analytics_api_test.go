package API_tests

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAnalyticsOccupancyEndpoint(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	// Seed a zone + capacity snapshot so the endpoint has a non-empty result set
	// and we can assert on the response shape (period/avg_occupancy_pct/peak_occupancy_pct).
	fx := createReservationFixture(t, env, admin, 10, 15)
	_, err := env.pool.Exec(context.Background(),
		`INSERT INTO capacity_snapshots(zone_id, snapshot_at, authoritative_stalls) VALUES ($1::uuid, now(), 7)`,
		fx.zoneID,
	)
	if err != nil {
		t.Fatalf("seed capacity_snapshot: %v", err)
	}

	from := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)

	resp := apiRequest(t, env.r, http.MethodGet, "/api/analytics/occupancy?from="+from+"&to="+to, nil, admin)
	logStep(t, "GET", "/api/analytics/occupancy", resp.Code, resp.Body.String())
	if resp.Code != http.StatusOK {
		t.Fatalf("analytics occupancy failed: %d %s", resp.Code, resp.Body.String())
	}

	var rows []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode occupancy response: %v body=%s", err, resp.Body.String())
	}
	if len(rows) == 0 {
		t.Fatalf("expected at least one occupancy bucket, got empty array")
	}
	first := rows[0]
	for _, key := range []string{"period", "avg_occupancy_pct", "peak_occupancy_pct"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("occupancy row missing field %q; got %v", key, first)
		}
	}
}

func TestAnalyticsBookingsEndpoint(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	// Seed a reservation so the bookings pivot has a non-empty result we can
	// assert on (label/count/total_stalls).
	fx := createReservationFixture(t, env, admin, 5, 15)
	start := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	end := start.Add(time.Hour)
	hold := apiRequest(t, env.r, http.MethodPost, "/api/reservations/hold", map[string]any{
		"zone_id":           fx.zoneID,
		"member_id":         fx.memberID,
		"vehicle_id":        fx.vehicleID,
		"time_window_start": start.Format(time.RFC3339),
		"time_window_end":   end.Format(time.RFC3339),
		"stall_count":       1,
	}, admin)
	if hold.Code != http.StatusCreated {
		t.Fatalf("seed hold: %d %s", hold.Code, hold.Body.String())
	}

	resp := apiRequest(t, env.r, http.MethodGet, "/api/analytics/bookings?pivot_by=time", nil, admin)
	logStep(t, "GET", "/api/analytics/bookings", resp.Code, resp.Body.String())
	if resp.Code != http.StatusOK {
		t.Fatalf("analytics bookings failed: %d %s", resp.Code, resp.Body.String())
	}

	var rows []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode bookings response: %v body=%s", err, resp.Body.String())
	}
	if len(rows) == 0 {
		t.Fatalf("expected at least one bookings bucket, got empty array")
	}
	first := rows[0]
	for _, key := range []string{"label", "count", "total_stalls"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("bookings row missing field %q; got %v", key, first)
		}
	}
	// count should be numeric > 0 because we seeded one reservation
	if c, ok := first["count"].(float64); !ok || c < 1 {
		t.Fatalf("expected count >= 1, got %v", first["count"])
	}
}

func TestAnalyticsExceptionsEndpoint(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	// Seed an exception so the pivot has a row to assert against.
	org := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO devices(id, organization_id, device_key, device_type, status)
		VALUES ('cc000001-0000-0000-0000-000000000099', $1::uuid, 'ANX-COV-1', 'camera', 'online')
		ON CONFLICT DO NOTHING
	`, org)
	if err != nil {
		t.Fatalf("seed device: %v", err)
	}
	_, err = env.pool.Exec(context.Background(), `
		INSERT INTO exceptions(device_id, exception_type, status, created_at)
		VALUES ('cc000001-0000-0000-0000-000000000099'::uuid, 'sensor_offline', 'open', now())
	`)
	if err != nil {
		t.Fatalf("seed exception: %v", err)
	}

	resp := apiRequest(t, env.r, http.MethodGet, "/api/analytics/exceptions", nil, admin)
	logStep(t, "GET", "/api/analytics/exceptions", resp.Code, resp.Body.String())
	if resp.Code != http.StatusOK {
		t.Fatalf("analytics exceptions failed: %d %s", resp.Code, resp.Body.String())
	}

	var rows []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode exceptions response: %v body=%s", err, resp.Body.String())
	}
	if len(rows) == 0 {
		t.Fatalf("expected at least one exceptions bucket, got empty array")
	}
	first := rows[0]
	for _, key := range []string{"exception_type", "count"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("exceptions row missing field %q; got %v", key, first)
		}
	}
	if c, ok := first["count"].(float64); !ok || c < 1 {
		t.Fatalf("expected exception count >= 1, got %v", first["count"])
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

func TestExportExcelCreateAndDownloadBinary(t *testing.T) {
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
	logStep(t, "GET", "/api/exports/:id/download (excel)", download.Code, "")
	if download.Code != http.StatusOK {
		t.Fatalf("download excel export failed: %d", download.Code)
	}
	ct := download.Result().Header.Get("Content-Type")
	if !strings.Contains(ct, "spreadsheetml") {
		t.Fatalf("expected excel content type, got %s", ct)
	}
	disp := download.Result().Header.Get("Content-Disposition")
	if !strings.Contains(disp, "export.xlsx") {
		t.Fatalf("expected export.xlsx filename, got %s", disp)
	}
	// Verify real XLSX binary: ZIP format starts with PK (0x50 0x4B)
	body := download.Body.Bytes()
	if len(body) < 4 || body[0] != 0x50 || body[1] != 0x4B {
		t.Fatalf("XLSX output is not a real ZIP/XLSX file (first bytes: %x)", body[:min(4, len(body))])
	}
}

func TestExportPDFCreateAndDownloadBinary(t *testing.T) {
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
	logStep(t, "GET", "/api/exports/:id/download (pdf)", download.Code, "")
	if download.Code != http.StatusOK {
		t.Fatalf("download pdf export failed: %d", download.Code)
	}
	ct := download.Result().Header.Get("Content-Type")
	if !strings.Contains(ct, "pdf") {
		t.Fatalf("expected pdf content type, got %s", ct)
	}
	disp := download.Result().Header.Get("Content-Disposition")
	if !strings.Contains(disp, "export.pdf") {
		t.Fatalf("expected export.pdf filename, got %s", disp)
	}
	// Verify real PDF binary: starts with %PDF
	body := download.Body.Bytes()
	if len(body) < 5 || string(body[:5]) != "%PDF-" {
		t.Fatalf("PDF output is not a real PDF file (first bytes: %q)", string(body[:min(10, len(body))]))
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

func TestExportSegmentStrictDeniedNoMatchingMembers(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	operator := loginAs(t, env, "operator", "UserPass1234")

	// Create a member in org A (operator's org) with 0 arrears
	apiRequest(t, env.r, http.MethodPost, "/api/members", map[string]any{
		"display_name": "Zero Arrears", "contact_notes": "n",
	}, admin)

	// Create segment that matches only members with arrears > 999999 (nobody matches)
	createSeg := apiRequest(t, env.r, http.MethodPost, "/api/segments", map[string]any{
		"name":              "No Match Segment",
		"filter_expression": map[string]any{"arrears_balance_cents": map[string]any{"gt": 999999}},
		"schedule":          "manual",
	}, admin)
	if createSeg.Code != http.StatusCreated {
		t.Fatalf("create segment: %d %s", createSeg.Code, createSeg.Body.String())
	}
	segmentID := extractID(t, createSeg.Body.String())

	// Operator should be denied — segment evaluates to empty set, no org members in scope
	createExport := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format":     "csv",
		"scope":      "bookings",
		"segment_id": segmentID,
	}, operator)
	logStep(t, "POST", "/api/exports (strict segment deny)", createExport.Code, createExport.Body.String())
	if createExport.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for operator with no matching segment members, got %d", createExport.Code)
	}
}

func TestExportSegmentStrictAllowedMatchingMembers(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")
	operator := loginAs(t, env, "operator", "UserPass1234")

	// Create a member in org A (operator's org) with high arrears
	apiRequest(t, env.r, http.MethodPost, "/api/members", map[string]any{
		"display_name": "High Arrears", "contact_notes": "n",
	}, admin)
	// Set arrears for the member
	_, _ = env.pool.Exec(context.Background(),
		`UPDATE members SET arrears_balance_cents = 10000 WHERE display_name = 'High Arrears'`)

	// Create segment that matches members with arrears > 5000
	createSeg := apiRequest(t, env.r, http.MethodPost, "/api/segments", map[string]any{
		"name":              "Match Segment",
		"filter_expression": map[string]any{"arrears_balance_cents": map[string]any{"gt": 5000}},
		"schedule":          "manual",
	}, admin)
	if createSeg.Code != http.StatusCreated {
		t.Fatalf("create segment: %d %s", createSeg.Code, createSeg.Body.String())
	}
	segmentID := extractID(t, createSeg.Body.String())

	// Operator should be allowed — segment result contains members from their org
	createExport := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format":     "csv",
		"scope":      "bookings",
		"segment_id": segmentID,
	}, operator)
	logStep(t, "POST", "/api/exports (strict segment allow)", createExport.Code, createExport.Body.String())
	if createExport.Code != http.StatusCreated {
		t.Fatalf("expected 201 for operator with matching segment members, got %d %s", createExport.Code, createExport.Body.String())
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

func TestExportSegmentRowFilteringOnlyIncludesSegmentMembers(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	// Create shared infrastructure: facility -> lot -> zone
	fac := apiRequest(t, env.r, http.MethodPost, "/api/facilities", map[string]any{"name": "SegFilter-Fac", "address": "X"}, admin)
	if fac.Code != http.StatusCreated {
		t.Fatalf("facility: %d %s", fac.Code, fac.Body.String())
	}
	facID := extractID(t, fac.Body.String())
	lot := apiRequest(t, env.r, http.MethodPost, "/api/lots", map[string]any{"facility_id": facID, "name": "SegFilter-Lot"}, admin)
	if lot.Code != http.StatusCreated {
		t.Fatalf("lot: %d %s", lot.Code, lot.Body.String())
	}
	lotID := extractID(t, lot.Body.String())
	zone := apiRequest(t, env.r, http.MethodPost, "/api/zones", map[string]any{"lot_id": lotID, "name": "SegFilter-Zone", "total_stalls": 50, "hold_timeout_minutes": 15}, admin)
	if zone.Code != http.StatusCreated {
		t.Fatalf("zone: %d %s", zone.Code, zone.Body.String())
	}
	zoneID := extractID(t, zone.Body.String())

	// Create two members: one with high arrears (in segment), one with zero (out of segment)
	mIn := apiRequest(t, env.r, http.MethodPost, "/api/members", map[string]any{"display_name": "InSegment", "contact_notes": "n"}, admin)
	if mIn.Code != http.StatusCreated {
		t.Fatalf("member in: %d %s", mIn.Code, mIn.Body.String())
	}
	memberInID := extractID(t, mIn.Body.String())

	mOut := apiRequest(t, env.r, http.MethodPost, "/api/members", map[string]any{"display_name": "OutSegment", "contact_notes": "n"}, admin)
	if mOut.Code != http.StatusCreated {
		t.Fatalf("member out: %d %s", mOut.Code, mOut.Body.String())
	}
	memberOutID := extractID(t, mOut.Body.String())

	// Set arrears: memberIn=20000, memberOut=0
	_, _ = env.pool.Exec(context.Background(), `UPDATE members SET arrears_balance_cents = 20000 WHERE id = $1::uuid`, memberInID)

	// Create vehicles for each member
	vIn := apiRequest(t, env.r, http.MethodPost, "/api/vehicles", map[string]any{"plate_number": "SEG-IN-1", "make": "A", "model": "B"}, admin)
	if vIn.Code != http.StatusCreated {
		t.Fatalf("vehicle in: %d %s", vIn.Code, vIn.Body.String())
	}
	vInID := extractID(t, vIn.Body.String())
	vOut := apiRequest(t, env.r, http.MethodPost, "/api/vehicles", map[string]any{"plate_number": "SEG-OUT-1", "make": "C", "model": "D"}, admin)
	if vOut.Code != http.StatusCreated {
		t.Fatalf("vehicle out: %d %s", vOut.Code, vOut.Body.String())
	}
	vOutID := extractID(t, vOut.Body.String())

	// Create reservations for each member
	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	end := start.Add(2 * time.Hour)
	holdIn := apiRequest(t, env.r, http.MethodPost, "/api/reservations/hold", map[string]any{
		"zone_id": zoneID, "member_id": memberInID, "vehicle_id": vInID,
		"time_window_start": start.Format(time.RFC3339), "time_window_end": end.Format(time.RFC3339), "stall_count": 1,
	}, admin)
	if holdIn.Code != http.StatusCreated {
		t.Fatalf("hold in: %d %s", holdIn.Code, holdIn.Body.String())
	}
	holdInID := extractID(t, holdIn.Body.String())

	holdOut := apiRequest(t, env.r, http.MethodPost, "/api/reservations/hold", map[string]any{
		"zone_id": zoneID, "member_id": memberOutID, "vehicle_id": vOutID,
		"time_window_start": start.Format(time.RFC3339), "time_window_end": end.Format(time.RFC3339), "stall_count": 1,
	}, admin)
	if holdOut.Code != http.StatusCreated {
		t.Fatalf("hold out: %d %s", holdOut.Code, holdOut.Body.String())
	}
	holdOutID := extractID(t, holdOut.Body.String())

	// Create segment matching only members with arrears > 10000
	seg := apiRequest(t, env.r, http.MethodPost, "/api/segments", map[string]any{
		"name":              "RowFilter Segment",
		"filter_expression": map[string]any{"arrears_balance_cents": map[string]any{"gt": 10000}},
		"schedule":          "manual",
	}, admin)
	if seg.Code != http.StatusCreated {
		t.Fatalf("segment: %d %s", seg.Code, seg.Body.String())
	}
	segID := extractID(t, seg.Body.String())

	// Create CSV export with segment_id
	exp := apiRequest(t, env.r, http.MethodPost, "/api/exports", map[string]any{
		"format": "csv", "scope": "bookings", "segment_id": segID,
	}, admin)
	logStep(t, "POST", "/api/exports (segment row filter)", exp.Code, exp.Body.String())
	if exp.Code != http.StatusCreated {
		t.Fatalf("export: %d %s", exp.Code, exp.Body.String())
	}
	exportID := extractID(t, exp.Body.String())

	// Download and verify: only the in-segment member's reservation should appear
	dl := apiRequest(t, env.r, http.MethodGet, "/api/exports/"+exportID+"/download", nil, admin)
	if dl.Code != http.StatusOK {
		t.Fatalf("download: %d", dl.Code)
	}
	body := dl.Body.String()
	if !strings.Contains(body, holdInID) {
		t.Fatalf("exported CSV should contain in-segment reservation %s, got:\n%s", holdInID, body)
	}
	if strings.Contains(body, holdOutID) {
		t.Fatalf("exported CSV should NOT contain out-of-segment reservation %s, got:\n%s", holdOutID, body)
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
