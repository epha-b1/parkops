package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"parkops/internal/auth"
)

var (
	cryptoRandRead     = rand.Read
	hexEncodeToString  = hex.EncodeToString
)

type masterHandler struct {
	pool          *pgxpool.Pool
	authService   *auth.Service
	encryptionKey []byte
}

func registerMasterDataRoutes(r *gin.Engine, authService *auth.Service, pool *pgxpool.Pool, encryptionKey []byte) {
	h := &masterHandler{pool: pool, authService: authService, encryptionKey: encryptionKey}

	allRead := []string{auth.RoleFacilityAdmin, auth.RoleFleetManager, auth.RoleDispatch, auth.RoleAuditor}

	read := r.Group("/api")
	read.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, allRead...))
	{
		read.GET("/facilities", h.listFacilities)
		read.GET("/facilities/:id", h.getFacility)
		read.GET("/lots", h.listLots)
		read.GET("/lots/:id", h.getLot)
		read.GET("/zones", h.listZones)
		read.GET("/zones/:id", h.getZone)
		read.GET("/rate-plans", h.listRatePlans)
		read.GET("/rate-plans/:id", h.getRatePlan)
		read.GET("/members", h.listMembers)
		read.GET("/members/:id", h.getMember)
		read.GET("/members/:id/balance", h.getMemberBalance)
		read.GET("/vehicles", h.listVehicles)
		read.GET("/vehicles/:id", h.getVehicle)
		read.GET("/drivers", h.listDrivers)
		read.GET("/drivers/:id", h.getDriver)
		read.GET("/message-rules", h.listMessageRules)
	}

	adminWrite := r.Group("/api")
	adminWrite.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, auth.RoleFacilityAdmin))
	{
		adminWrite.POST("/facilities", h.createFacility)
		adminWrite.PATCH("/facilities/:id", h.updateFacility)
		adminWrite.DELETE("/facilities/:id", h.deleteFacility)
		adminWrite.POST("/lots", h.createLot)
		adminWrite.PATCH("/lots/:id", h.updateLot)
		adminWrite.DELETE("/lots/:id", h.deleteLot)
		adminWrite.POST("/zones", h.createZone)
		adminWrite.PATCH("/zones/:id", h.updateZone)
		adminWrite.DELETE("/zones/:id", h.deleteZone)
		adminWrite.POST("/rate-plans", h.createRatePlan)
		adminWrite.PATCH("/rate-plans/:id", h.updateRatePlan)
		adminWrite.DELETE("/rate-plans/:id", h.deleteRatePlan)
		adminWrite.PATCH("/members/:id/balance", h.patchMemberBalance)
		adminWrite.POST("/message-rules", h.createMessageRule)
		adminWrite.PATCH("/message-rules/:id", h.updateMessageRule)
		adminWrite.DELETE("/message-rules/:id", h.deleteMessageRule)
	}

	fleetWrite := r.Group("/api")
	fleetWrite.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, auth.RoleFacilityAdmin, auth.RoleFleetManager))
	{
		fleetWrite.POST("/members", h.createMember)
		fleetWrite.PATCH("/members/:id", h.updateMember)
		fleetWrite.DELETE("/members/:id", h.deleteMember)
		fleetWrite.POST("/vehicles", h.createVehicle)
		fleetWrite.PATCH("/vehicles/:id", h.updateVehicle)
		fleetWrite.DELETE("/vehicles/:id", h.deleteVehicle)
		fleetWrite.POST("/drivers", h.createDriver)
		fleetWrite.PATCH("/drivers/:id", h.updateDriver)
		fleetWrite.DELETE("/drivers/:id", h.deleteDriver)
	}
}

func actor(c *gin.Context) (auth.User, bool) { return getCurrentUser(c) }

func (h *masterHandler) isCrossOrgForbidden(c *gin.Context, organizationID string) bool {
	u, ok := actor(c)
	if !ok {
		return true
	}
	if auth.HasAnyRole(u.Roles, []string{auth.RoleFacilityAdmin}) {
		return false
	}
	if auth.HasAnyRole(u.Roles, []string{auth.RoleFleetManager}) {
		if u.OrganizationID == nil || *u.OrganizationID != organizationID {
			abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
			return true
		}
	}
	return false
}

func (h *masterHandler) assertOrgScopeByID(c *gin.Context, table, id string) bool {
	if !isValidUUID(id) {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid UUID format")
		return false
	}
	u, ok := actor(c)
	if !ok {
		abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return false
	}
	if auth.HasAnyRole(u.Roles, []string{auth.RoleFacilityAdmin}) {
		return true
	}
	if !auth.HasAnyRole(u.Roles, []string{auth.RoleFleetManager}) {
		abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
	var orgID string
	err := h.pool.QueryRow(c.Request.Context(), "SELECT organization_id::text FROM "+table+" WHERE id=$1", id).Scan(&orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "resource not found")
			return false
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return false
	}
	if u.OrganizationID == nil || *u.OrganizationID != orgID {
		abortAPIError(c, 403, "FORBIDDEN", "forbidden")
		return false
	}
	return true
}

func parseIntDefault(v string, d int) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return d
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return d
	}
	return i
}

// Many handlers below keep logic simple and explicit for reliability.

func (h *masterHandler) listFacilities(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(), `SELECT id::text, name, COALESCE(address,''), created_at FROM facilities ORDER BY created_at DESC`)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var id, name, address string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &address, &createdAt); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		out = append(out, gin.H{"id": id, "name": name, "address": address, "created_at": createdAt.UTC().Format(timeRFC3339)})
	}
	c.JSON(200, out)
}

func (h *masterHandler) getFacility(c *gin.Context) {
	h.getByID(c, "facilities", []string{"name", "address", "created_at"})
}
func (h *masterHandler) getLot(c *gin.Context) { h.getByID(c, "lots", []string{"facility_id", "name"}) }
func (h *masterHandler) getZone(c *gin.Context) {
	h.getByID(c, "zones", []string{"lot_id", "name", "total_stalls", "hold_timeout_minutes"})
}
func (h *masterHandler) getRatePlan(c *gin.Context) {
	h.getByID(c, "rate_plans", []string{"zone_id", "name", "rate_cents", "period"})
}

func (h *masterHandler) createFacility(c *gin.Context) {
	var b struct{ Name, Address string }
	if c.ShouldBindJSON(&b) != nil || strings.TrimSpace(b.Name) == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	h.insertSimple(c, "facilities", []string{"name", "address"}, []any{b.Name, b.Address})
}

func (h *masterHandler) updateFacility(c *gin.Context) {
	var b struct{ Name, Address string }
	if c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	h.patchSimple(c, "facilities", map[string]any{"name": b.Name, "address": b.Address})
}

func (h *masterHandler) deleteFacility(c *gin.Context) { h.deleteSimple(c, "facilities") }

func (h *masterHandler) listLots(c *gin.Context) {
	facilityID := c.Query("facility_id")
	if strings.TrimSpace(facilityID) != "" {
		if !isValidUUID(facilityID) {
			abortAPIError(c, 400, "VALIDATION_ERROR", "facility_id must be a valid UUID")
			return
		}
		h.listSimple(c, `SELECT id::text, facility_id::text, name FROM lots WHERE facility_id=$1 ORDER BY name`, facilityID)
		return
	}
	h.listSimple(c, `SELECT id::text, facility_id::text, name FROM lots ORDER BY name`)
}

func (h *masterHandler) createLot(c *gin.Context) {
	var b struct {
		FacilityID string `json:"facility_id"`
		Name       string `json:"name"`
	}
	if c.ShouldBindJSON(&b) != nil || b.FacilityID == "" || b.Name == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if !isValidUUID(b.FacilityID) {
		abortAPIError(c, 400, "VALIDATION_ERROR", "facility_id must be a valid UUID")
		return
	}
	h.insertSimple(c, "lots", []string{"facility_id", "name"}, []any{b.FacilityID, b.Name})
}

func (h *masterHandler) updateLot(c *gin.Context) {
	var b struct{ Name string }
	if c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	h.patchSimple(c, "lots", map[string]any{"name": b.Name})
}

func (h *masterHandler) deleteLot(c *gin.Context) { h.deleteSimple(c, "lots") }

func (h *masterHandler) listZones(c *gin.Context) {
	lotID := c.Query("lot_id")
	if strings.TrimSpace(lotID) != "" {
		if !isValidUUID(lotID) {
			abortAPIError(c, 400, "VALIDATION_ERROR", "lot_id must be a valid UUID")
			return
		}
		h.listSimple(c, `SELECT id::text, lot_id::text, name, total_stalls, hold_timeout_minutes FROM zones WHERE lot_id=$1 ORDER BY name`, lotID)
		return
	}
	h.listSimple(c, `SELECT id::text, lot_id::text, name, total_stalls, hold_timeout_minutes FROM zones ORDER BY name`)
}

func (h *masterHandler) createZone(c *gin.Context) {
	var b struct {
		LotID              string `json:"lot_id"`
		Name               string `json:"name"`
		TotalStalls        int    `json:"total_stalls"`
		HoldTimeoutMinutes int    `json:"hold_timeout_minutes"`
	}
	if c.ShouldBindJSON(&b) != nil || b.LotID == "" || b.Name == "" || b.TotalStalls <= 0 {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if !isValidUUID(b.LotID) {
		abortAPIError(c, 400, "VALIDATION_ERROR", "lot_id must be a valid UUID")
		return
	}
	if b.HoldTimeoutMinutes <= 0 {
		b.HoldTimeoutMinutes = 15
	}
	h.insertSimple(c, "zones", []string{"lot_id", "name", "total_stalls", "hold_timeout_minutes"}, []any{b.LotID, b.Name, b.TotalStalls, b.HoldTimeoutMinutes})
}

func (h *masterHandler) updateZone(c *gin.Context) {
	var b struct {
		Name               string `json:"name"`
		TotalStalls        int    `json:"total_stalls"`
		HoldTimeoutMinutes int    `json:"hold_timeout_minutes"`
	}
	if c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if b.TotalStalls < 0 || b.HoldTimeoutMinutes < 0 {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}

	if b.TotalStalls > 0 {
		maxDemand, err := maxConfirmedDemand(c, h.pool, c.Param("id"))
		if err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		if b.TotalStalls < maxDemand {
			abortAPIError(c, 409, "CAPACITY_CONFLICT", "zone total stalls cannot go below confirmed reservations")
			return
		}
	}

	h.patchSimple(c, "zones", map[string]any{"name": b.Name, "total_stalls": b.TotalStalls, "hold_timeout_minutes": b.HoldTimeoutMinutes})
}

func (h *masterHandler) deleteZone(c *gin.Context) { h.deleteSimple(c, "zones") }

func (h *masterHandler) listRatePlans(c *gin.Context) {
	zoneID := c.Query("zone_id")
	if strings.TrimSpace(zoneID) != "" {
		if !isValidUUID(zoneID) {
			abortAPIError(c, 400, "VALIDATION_ERROR", "zone_id must be a valid UUID")
			return
		}
		h.listSimple(c, `SELECT id::text, zone_id::text, name, rate_cents, period FROM rate_plans WHERE zone_id=$1 ORDER BY name`, zoneID)
		return
	}
	h.listSimple(c, `SELECT id::text, zone_id::text, name, rate_cents, period FROM rate_plans ORDER BY name`)
}

func (h *masterHandler) createRatePlan(c *gin.Context) {
	var b struct {
		ZoneID    string `json:"zone_id"`
		Name      string `json:"name"`
		RateCents int    `json:"rate_cents"`
		Period    string `json:"period"`
	}
	if c.ShouldBindJSON(&b) != nil || b.ZoneID == "" || b.Name == "" || b.RateCents < 0 {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if !isValidUUID(b.ZoneID) {
		abortAPIError(c, 400, "VALIDATION_ERROR", "zone_id must be a valid UUID")
		return
	}
	h.insertSimple(c, "rate_plans", []string{"zone_id", "name", "rate_cents", "period"}, []any{b.ZoneID, b.Name, b.RateCents, b.Period})
}

func (h *masterHandler) updateRatePlan(c *gin.Context) {
	var b struct {
		Name      string `json:"name"`
		RateCents int    `json:"rate_cents"`
		Period    string `json:"period"`
	}
	if c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	h.patchSimple(c, "rate_plans", map[string]any{"name": b.Name, "rate_cents": b.RateCents, "period": b.Period})
}

func (h *masterHandler) deleteRatePlan(c *gin.Context) { h.deleteSimple(c, "rate_plans") }

// shared helpers
func (h *masterHandler) actorOrgOrDefault(c *gin.Context) string {
	u, ok := actor(c)
	if ok && u.OrganizationID != nil {
		return *u.OrganizationID
	}
	return "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
}

func (h *masterHandler) getByID(c *gin.Context, table string, fields []string) {
	cols := "id::text"
	for _, f := range fields {
		cols += ", " + f
	}
	row := h.pool.QueryRow(c.Request.Context(), "SELECT "+cols+" FROM "+table+" WHERE id=$1", c.Param("id"))
	vals := make([]any, len(fields)+1)
	vals[0] = new(string)
	for i := range fields {
		vals[i+1] = new(any)
	}
	if err := row.Scan(vals...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "resource not found")
		} else {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	out := gin.H{"id": *(vals[0].(*string))}
	for i, f := range fields {
		out[f] = *(vals[i+1].(*any))
	}
	c.JSON(200, out)
}

func (h *masterHandler) insertSimple(c *gin.Context, table string, cols []string, args []any) {
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = "$" + strconv.Itoa(i+1)
	}
	query := "INSERT INTO " + table + "(" + strings.Join(cols, ",") + ") VALUES (" + strings.Join(placeholders, ",") + ") RETURNING id::text"
	var id string
	if err := h.pool.QueryRow(c.Request.Context(), query, args...).Scan(&id); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(201, gin.H{"id": id})
}

func (h *masterHandler) patchSimple(c *gin.Context, table string, fields map[string]any) {
	set := make([]string, 0)
	args := make([]any, 0)
	i := 1
	for k, v := range fields {
		s := strings.TrimSpace(k)
		switch vv := v.(type) {
		case string:
			if strings.TrimSpace(vv) == "" {
				continue
			}
		}
		set = append(set, s+"=$"+strconv.Itoa(i))
		args = append(args, v)
		i++
	}
	if len(set) == 0 {
		abortAPIError(c, 400, "VALIDATION_ERROR", "no fields to update")
		return
	}
	args = append(args, c.Param("id"))
	query := "UPDATE " + table + " SET " + strings.Join(set, ",") + ", updated_at=now() WHERE id=$" + strconv.Itoa(i)
	ct, err := h.pool.Exec(c.Request.Context(), query, args...)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if ct.RowsAffected() == 0 {
		abortAPIError(c, 404, "NOT_FOUND", "resource not found")
		return
	}
	c.JSON(200, gin.H{"message": "updated"})
}

func (h *masterHandler) deleteSimple(c *gin.Context, table string) {
	_, err := uuid.Parse(c.Param("id"))
	if err != nil {
		abortAPIError(c, 404, "NOT_FOUND", "resource not found")
		return
	}
	ct, err := h.pool.Exec(c.Request.Context(), "DELETE FROM "+table+" WHERE id=$1", c.Param("id"))
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if ct.RowsAffected() == 0 {
		abortAPIError(c, 404, "NOT_FOUND", "resource not found")
		return
	}
	c.Status(204)
}

func (h *masterHandler) listSimple(c *gin.Context, query string, args ...any) {
	rows, err := h.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	fields := rows.FieldDescriptions()
	out := make([]gin.H, 0)
	for rows.Next() {
		vals := make([]any, len(fields))
		ptrs := make([]any, len(fields))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		m := gin.H{}
		for i, f := range fields {
			m[string(f.Name)] = vals[i]
		}
		out = append(out, m)
	}
	c.JSON(200, out)
}

func (h *masterHandler) listOrgScoped(c *gin.Context, table string, fields []string) {
	u, _ := actor(c)
	q := "SELECT id::text, organization_id::text"
	for _, f := range fields {
		q += ", " + f
	}
	q += " FROM " + table
	args := []any{}
	if auth.HasAnyRole(u.Roles, []string{auth.RoleFleetManager}) && u.OrganizationID != nil {
		q += " WHERE organization_id=$1"
		args = append(args, *u.OrganizationID)
	}
	q += " ORDER BY created_at DESC"
	h.listSimple(c, q, args...)
}

func (h *masterHandler) getOrgScoped(c *gin.Context, table string, fields []string) {
	q := "SELECT id::text, organization_id::text"
	for _, f := range fields {
		q += ", " + f
	}
	q += " FROM " + table + " WHERE id=$1"
	row := h.pool.QueryRow(c.Request.Context(), q, c.Param("id"))
	vals := make([]any, len(fields)+2)
	vals[0] = new(string)
	vals[1] = new(string)
	for i := range fields {
		vals[i+2] = new(any)
	}
	if err := row.Scan(vals...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "resource not found")
		} else {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	orgID := *(vals[1].(*string))
	if h.isCrossOrgForbidden(c, orgID) {
		return
	}
	out := gin.H{"id": *(vals[0].(*string)), "organization_id": orgID}
	for i, f := range fields {
		out[f] = *(vals[i+2].(*any))
	}
	c.JSON(200, out)
}
