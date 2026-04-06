package exports

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"parkops/internal/auth"
	"parkops/internal/segments"
)

// SegmentAuthorizer checks whether an actor has access to a segment-scoped export.
type SegmentAuthorizer struct {
	pool           *pgxpool.Pool
	segmentService *segments.Service
}

// NewSegmentAuthorizer creates a reusable authorizer for segment-scoped exports.
func NewSegmentAuthorizer(pool *pgxpool.Pool, segmentService *segments.Service) *SegmentAuthorizer {
	return &SegmentAuthorizer{pool: pool, segmentService: segmentService}
}

// CheckAccess enforces role AND segment-result membership.
// Admins bypass the membership check (only segment existence is verified).
// Non-admins must have members from their organization in the evaluated segment result set.
func (a *SegmentAuthorizer) CheckAccess(ctx context.Context, actor auth.User, segmentID string) (bool, error) {
	if auth.HasAnyRole(actor.Roles, []string{auth.RoleFacilityAdmin}) {
		var exists bool
		err := a.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM segment_definitions WHERE id=$1::uuid)`, segmentID).Scan(&exists)
		if err != nil {
			return false, err
		}
		return exists, nil
	}
	return a.isActorInSegmentScope(ctx, actor, segmentID)
}

// ResolveMembers evaluates a segment and returns the matching member IDs.
// Useful for data-scope filtering in exports.
func (a *SegmentAuthorizer) ResolveMembers(ctx context.Context, segmentID string) ([]string, error) {
	var filterExpr json.RawMessage
	err := a.pool.QueryRow(ctx,
		`SELECT filter_expression FROM segment_definitions WHERE id=$1::uuid`, segmentID).Scan(&filterExpr)
	if err != nil {
		return nil, err
	}
	return a.segmentService.EvaluateSegment(ctx, filterExpr)
}

// isActorInSegmentScope strictly evaluates the segment's filter expression and
// checks whether any resulting member belongs to the actor's organization.
func (a *SegmentAuthorizer) isActorInSegmentScope(ctx context.Context, actor auth.User, segmentID string) (bool, error) {
	if actor.OrganizationID == nil {
		return false, nil
	}
	var filterExpr json.RawMessage
	err := a.pool.QueryRow(ctx,
		`SELECT filter_expression FROM segment_definitions WHERE id=$1::uuid`, segmentID).Scan(&filterExpr)
	if err != nil {
		return false, nil
	}
	memberIDs, err := a.segmentService.EvaluateSegment(ctx, filterExpr)
	if err != nil || len(memberIDs) == 0 {
		return false, nil
	}
	var inScope bool
	err = a.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM members m
			WHERE m.id = ANY($1::uuid[])
			AND m.organization_id = $2::uuid
		)
	`, memberIDs, *actor.OrganizationID).Scan(&inScope)
	if err != nil {
		return false, nil
	}
	return inScope, nil
}
