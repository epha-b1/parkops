package server

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"parkops/internal/auth"
	"parkops/internal/platform/security"
)

func (h *masterHandler) listMembers(c *gin.Context) {
	u, _ := actor(c)
	page := parseIntDefault(c.Query("page"), 1)
	limit := parseIntDefault(c.Query("limit"), 20)
	offset := (page - 1) * limit

	base := `SELECT id::text, organization_id::text, display_name, arrears_balance_cents, created_at, contact_notes_enc FROM members`
	args := []any{}
	if auth.HasAnyRole(u.Roles, []string{auth.RoleFleetManager}) && u.OrganizationID != nil {
		base += ` WHERE organization_id = $1`
		args = append(args, *u.OrganizationID)
	}
	base += ` ORDER BY created_at DESC OFFSET $` + strconv.Itoa(len(args)+1) + ` LIMIT $` + strconv.Itoa(len(args)+2)
	args = append(args, offset, limit)

	rows, err := h.pool.Query(c.Request.Context(), base, args...)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, orgID, name string
		var balance int
		var enc sql.NullString
		var created time.Time
		if err := rows.Scan(&id, &orgID, &name, &balance, &created, &enc); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		items = append(items, gin.H{"id": id, "organization_id": orgID, "display_name": name, "arrears_balance_cents": balance, "created_at": created.UTC().Format(timeRFC3339)})
	}
	c.JSON(200, gin.H{"items": items, "total": len(items)})
}

func (h *masterHandler) createMember(c *gin.Context) {
	var b struct {
		DisplayName  string `json:"display_name"`
		ContactNotes string `json:"contact_notes"`
	}
	if c.ShouldBindJSON(&b) != nil || strings.TrimSpace(b.DisplayName) == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	enc, err := security.EncryptString(h.encryptionKey, b.ContactNotes)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	orgID := h.actorOrgOrDefault(c)
	row := h.pool.QueryRow(c.Request.Context(), `INSERT INTO members(organization_id,display_name,contact_notes_enc) VALUES ($1,$2,$3) RETURNING id::text, organization_id::text, display_name, arrears_balance_cents, created_at`, orgID, b.DisplayName, enc)
	var id, org, name string
	var bal int
	var created time.Time
	if err := row.Scan(&id, &org, &name, &bal, &created); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(201, gin.H{"id": id, "organization_id": org, "display_name": name, "arrears_balance_cents": bal, "created_at": created.UTC().Format(timeRFC3339)})
}

func (h *masterHandler) getMember(c *gin.Context) {
	var id, orgID, name string
	var bal int
	var enc sql.NullString
	var created time.Time
	err := h.pool.QueryRow(c.Request.Context(), `SELECT id::text, organization_id::text, display_name, arrears_balance_cents, created_at, contact_notes_enc FROM members WHERE id=$1`, c.Param("id")).Scan(&id, &orgID, &name, &bal, &created, &enc)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "resource not found")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if h.isCrossOrgForbidden(c, orgID) {
		return
	}
	c.JSON(200, gin.H{"id": id, "organization_id": orgID, "display_name": name, "arrears_balance_cents": bal, "created_at": created.UTC().Format(timeRFC3339)})
}

func (h *masterHandler) updateMember(c *gin.Context) {
	if !h.assertOrgScopeByID(c, "members", c.Param("id")) {
		return
	}
	var b struct {
		DisplayName  string `json:"display_name"`
		ContactNotes string `json:"contact_notes"`
	}
	if c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	enc := ""
	if b.ContactNotes != "" {
		v, err := security.EncryptString(h.encryptionKey, b.ContactNotes)
		if err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		enc = v
	}
	_, err := h.pool.Exec(c.Request.Context(), `UPDATE members SET display_name=COALESCE(NULLIF($2,''),display_name), contact_notes_enc=CASE WHEN $3='' THEN contact_notes_enc ELSE $3 END, updated_at=now() WHERE id=$1`, c.Param("id"), b.DisplayName, enc)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	h.getMember(c)
}

func (h *masterHandler) deleteMember(c *gin.Context) {
	if !h.assertOrgScopeByID(c, "members", c.Param("id")) {
		return
	}
	h.deleteSimple(c, "members")
}
func (h *masterHandler) getMemberBalance(c *gin.Context) {
	if !h.assertOrgScopeByID(c, "members", c.Param("id")) {
		return
	}
	var bal int
	err := h.pool.QueryRow(c.Request.Context(), `SELECT arrears_balance_cents FROM members WHERE id=$1`, c.Param("id")).Scan(&bal)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "resource not found")
		} else {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	c.JSON(200, gin.H{"arrears_balance_cents": bal})
}
func (h *masterHandler) patchMemberBalance(c *gin.Context) {
	if !h.assertOrgScopeByID(c, "members", c.Param("id")) {
		return
	}
	var b struct {
		AmountCents int    `json:"amount_cents"`
		Reason      string `json:"reason"`
	}
	if c.ShouldBindJSON(&b) != nil || strings.TrimSpace(b.Reason) == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	var bal int
	err := h.pool.QueryRow(c.Request.Context(), `UPDATE members SET arrears_balance_cents=arrears_balance_cents+$2, updated_at=now() WHERE id=$1 RETURNING arrears_balance_cents`, c.Param("id"), b.AmountCents).Scan(&bal)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "resource not found")
		} else {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	if a := getActorID(c); a != nil {
		id := c.Param("id")
		_ = h.authService.WriteAuditLog(c.Request.Context(), a, "member_balance_adjust", "member", &id, map[string]any{"amount_cents": b.AmountCents, "reason": b.Reason})
	}
	c.JSON(200, gin.H{"arrears_balance_cents": bal})
}

func (h *masterHandler) listVehicles(c *gin.Context) {
	h.listOrgScoped(c, "vehicles", []string{"plate_number", "make", "model"})
}
func (h *masterHandler) getVehicle(c *gin.Context) {
	h.getOrgScoped(c, "vehicles", []string{"plate_number", "make", "model"})
}
func (h *masterHandler) createVehicle(c *gin.Context) {
	var b struct {
		PlateNumber string `json:"plate_number"`
		Make        string `json:"make"`
		Model       string `json:"model"`
	}
	if c.ShouldBindJSON(&b) != nil || strings.TrimSpace(b.PlateNumber) == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	// Generate dedicated signing secret for HMAC trust
	secretBytes := make([]byte, 32)
	if _, err := cryptoRandRead(secretBytes); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	secretHex := hexEncodeToString(secretBytes)
	encSecret, err := security.EncryptString(h.encryptionKey, secretHex)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	h.insertSimple(c, "vehicles", []string{"organization_id", "plate_number", "make", "model", "signing_secret_enc"}, []any{h.actorOrgOrDefault(c), b.PlateNumber, b.Make, b.Model, encSecret})
}
func (h *masterHandler) updateVehicle(c *gin.Context) {
	if !h.assertOrgScopeByID(c, "vehicles", c.Param("id")) {
		return
	}
	var b struct {
		PlateNumber string `json:"plate_number"`
		Make        string `json:"make"`
		Model       string `json:"model"`
	}
	if c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	h.patchSimple(c, "vehicles", map[string]any{"plate_number": b.PlateNumber, "make": b.Make, "model": b.Model})
}
func (h *masterHandler) deleteVehicle(c *gin.Context) {
	if !h.assertOrgScopeByID(c, "vehicles", c.Param("id")) {
		return
	}
	h.deleteSimple(c, "vehicles")
}

func (h *masterHandler) listDrivers(c *gin.Context) {
	h.listOrgScoped(c, "drivers", []string{"member_id", "licence_number"})
}
func (h *masterHandler) getDriver(c *gin.Context) {
	h.getOrgScoped(c, "drivers", []string{"member_id", "licence_number"})
}
func (h *masterHandler) createDriver(c *gin.Context) {
	var b struct {
		MemberID      string `json:"member_id"`
		LicenceNumber string `json:"licence_number"`
	}
	if c.ShouldBindJSON(&b) != nil || b.MemberID == "" || b.LicenceNumber == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if !isValidUUID(b.MemberID) {
		abortAPIError(c, 400, "VALIDATION_ERROR", "member_id must be a valid UUID")
		return
	}
	if !h.assertOrgScopeByID(c, "members", b.MemberID) {
		return
	}
	h.insertSimple(c, "drivers", []string{"organization_id", "member_id", "licence_number"}, []any{h.actorOrgOrDefault(c), b.MemberID, b.LicenceNumber})
}
func (h *masterHandler) updateDriver(c *gin.Context) {
	if !h.assertOrgScopeByID(c, "drivers", c.Param("id")) {
		return
	}
	var b struct {
		LicenceNumber string `json:"licence_number"`
	}
	if c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	h.patchSimple(c, "drivers", map[string]any{"licence_number": b.LicenceNumber})
}
func (h *masterHandler) deleteDriver(c *gin.Context) {
	if !h.assertOrgScopeByID(c, "drivers", c.Param("id")) {
		return
	}
	h.deleteSimple(c, "drivers")
}

func (h *masterHandler) listMessageRules(c *gin.Context) {
	h.listSimple(c, `SELECT id::text, trigger_event, topic_id::text, template, active FROM message_rules ORDER BY created_at DESC`)
}
func (h *masterHandler) createMessageRule(c *gin.Context) {
	var b struct {
		TriggerEvent string `json:"trigger_event"`
		TopicID      string `json:"topic_id"`
		Template     string `json:"template"`
		Active       *bool  `json:"active"`
	}
	if c.ShouldBindJSON(&b) != nil || b.TriggerEvent == "" || b.TopicID == "" || b.Template == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if !isValidUUID(b.TopicID) {
		abortAPIError(c, 400, "VALIDATION_ERROR", "topic_id must be a valid UUID")
		return
	}
	active := true
	if b.Active != nil {
		active = *b.Active
	}
	h.insertSimple(c, "message_rules", []string{"trigger_event", "topic_id", "template", "active"}, []any{b.TriggerEvent, b.TopicID, b.Template, active})
}
func (h *masterHandler) updateMessageRule(c *gin.Context) {
	var b struct {
		Template string `json:"template"`
		Active   *bool  `json:"active"`
	}
	if c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	values := map[string]any{"template": b.Template}
	if b.Active != nil {
		values["active"] = *b.Active
	}
	h.patchSimple(c, "message_rules", values)
}
func (h *masterHandler) deleteMessageRule(c *gin.Context) { h.deleteSimple(c, "message_rules") }
