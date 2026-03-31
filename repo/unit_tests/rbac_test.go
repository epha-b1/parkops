package unit_tests

import (
	"testing"

	"parkops/internal/auth"
)

func TestRoleCheckLogic(t *testing.T) {
	if !auth.HasAnyRole([]string{auth.RoleFacilityAdmin}, []string{auth.RoleFacilityAdmin}) {
		t.Fatal("facility admin should be allowed")
	}
	if auth.HasAnyRole([]string{auth.RoleDispatch}, []string{auth.RoleFacilityAdmin}) {
		t.Fatal("dispatch should be blocked from admin-only")
	}
	if !auth.HasAnyRole([]string{auth.RoleAuditor}, []string{auth.RoleAuditor}) {
		t.Fatal("auditor should be allowed for audit route")
	}
	if auth.HasAnyRole([]string{auth.RoleFleetManager}, []string{auth.RoleAuditor}) {
		t.Fatal("fleet manager should be blocked for audit route")
	}
}
