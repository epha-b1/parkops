package auth

import "time"

const (
	SessionCookieName = "session_id"
	SessionTimeout    = 30 * time.Minute
	LockoutThreshold  = 5
	LockoutDuration   = 15 * time.Minute

	RoleFacilityAdmin = "facility_admin"
	RoleDispatch      = "dispatch_operator"
	RoleFleetManager  = "fleet_manager"
	RoleAuditor       = "auditor"
)

type User struct {
	ID                  string
	OrganizationID      *string
	Username            string
	PasswordHash        string
	DisplayName         string
	Status              string
	FailedLoginCount    int
	LockedUntil         *time.Time
	ForcePasswordChange bool
	CreatedAt           time.Time
	Roles               []string
}

type Session struct {
	ID           string
	UserID       string
	CreatedAt    time.Time
	LastActiveAt time.Time
	ExpiresAt    time.Time
}

type LoginResult struct {
	User    User
	Session Session
}

type AuditLog struct {
	ID           string
	ActorID      *string
	Action       string
	ResourceType string
	ResourceID   *string
	Detail       []byte
	CreatedAt    time.Time
}
