package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"parkops/internal/platform/security"
)

var (
	ErrUnauthorized      = errors.New("unauthorized")
	ErrRateLimited       = errors.New("rate_limited")
	ErrForbidden         = errors.New("forbidden")
	ErrValidation        = errors.New("validation")
	ErrNotFound          = errors.New("not_found")
	ErrPasswordIncorrect = errors.New("password_incorrect")
)

type Store interface {
	GetUserByUsername(ctx context.Context, username string) (User, error)
	GetUserByID(ctx context.Context, userID string) (User, error)
	IncrementFailedLogin(ctx context.Context, userID string) (int, error)
	SetLockedUntil(ctx context.Context, userID string, lockedUntil time.Time) (time.Time, error)
	ClearLoginFailures(ctx context.Context, userID string) error
	CreateSession(ctx context.Context, userID string, now, expiresAt time.Time) (Session, error)
	GetSessionByID(ctx context.Context, sessionID string) (Session, error)
	TouchSession(ctx context.Context, sessionID string, lastActive, expiresAt time.Time) error
	DeleteSession(ctx context.Context, sessionID string) error
	DeleteSessionsByUserID(ctx context.Context, userID string) error
	ListSessionsByUserID(ctx context.Context, userID string) ([]Session, error)
	UpdatePassword(ctx context.Context, userID, passwordHash string, forcePasswordChange bool) error
	UnlockUser(ctx context.Context, userID string) error
	ListUsers(ctx context.Context, offset, limit int) ([]User, error)
	CountUsers(ctx context.Context) (int, error)
	CreateUser(ctx context.Context, username, passwordHash, displayName string) (User, error)
	UpdateUser(ctx context.Context, userID, username, displayName, status string) (User, error)
	DeleteUser(ctx context.Context, userID string) error
	ReplaceUserRoles(ctx context.Context, userID string, roles []string) error
	ListAuditLogs(ctx context.Context, offset, limit int) ([]AuditLog, error)
	CountAuditLogs(ctx context.Context) (int, error)
	WriteAuditLog(ctx context.Context, actorID *string, action, resourceType string, resourceID *string, detail map[string]any) error
}

type LoginError struct {
	base              error
	AttemptsRemaining int
	LockedUntil       *time.Time
}

func (e *LoginError) Error() string {
	if e.base == nil {
		return "login failed"
	}
	return e.base.Error()
}

func (e *LoginError) Is(target error) bool {
	return e.base == target
}

type Service struct {
	store Store
	nowFn func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, nowFn: time.Now}
}

func (s *Service) SetNowFunc(nowFn func() time.Time) {
	s.nowFn = nowFn
}

func (s *Service) Login(ctx context.Context, username, password string) (LoginResult, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return LoginResult{}, ErrUnauthorized
	}

	now := s.nowFn().UTC()
	if user.LockedUntil != nil && now.Before(*user.LockedUntil) {
		return LoginResult{}, &LoginError{base: ErrRateLimited, LockedUntil: user.LockedUntil}
	}

	ok, err := security.VerifyPassword(user.PasswordHash, password)
	if err != nil {
		return LoginResult{}, ErrUnauthorized
	}

	if !ok {
		count, err := s.store.IncrementFailedLogin(ctx, user.ID)
		if err != nil {
			return LoginResult{}, err
		}
		attemptsRemaining := LockoutThreshold - count
		if attemptsRemaining < 0 {
			attemptsRemaining = 0
		}
		if count >= LockoutThreshold {
			lockedUntil := now.Add(LockoutDuration)
			storedLockedUntil, err := s.store.SetLockedUntil(ctx, user.ID, lockedUntil)
			if err != nil {
				return LoginResult{}, err
			}
			return LoginResult{}, &LoginError{base: ErrRateLimited, AttemptsRemaining: 0, LockedUntil: &storedLockedUntil}
		}
		return LoginResult{}, &LoginError{base: ErrUnauthorized, AttemptsRemaining: attemptsRemaining}
	}

	if err := s.store.ClearLoginFailures(ctx, user.ID); err != nil {
		return LoginResult{}, err
	}

	expires := now.Add(SessionTimeout)
	session, err := s.store.CreateSession(ctx, user.ID, now, expires)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{User: user, Session: session}, nil
}

func (s *Service) AuthenticateSession(ctx context.Context, sessionID string) (User, Session, error) {
	session, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return User{}, Session{}, ErrUnauthorized
	}

	now := s.nowFn().UTC()
	if IsSessionExpired(session.LastActiveAt, now, SessionTimeout) || now.After(session.ExpiresAt) {
		_ = s.store.DeleteSession(ctx, sessionID)
		return User{}, Session{}, ErrUnauthorized
	}

	newExpiry := now.Add(SessionTimeout)
	if err := s.store.TouchSession(ctx, session.ID, now, newExpiry); err != nil {
		return User{}, Session{}, err
	}
	session.LastActiveAt = now
	session.ExpiresAt = newExpiry

	user, err := s.store.GetUserByID(ctx, session.UserID)
	if err != nil {
		_ = s.store.DeleteSession(ctx, sessionID)
		return User{}, Session{}, ErrUnauthorized
	}

	return user, session, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	return s.store.DeleteSession(ctx, sessionID)
}

func (s *Service) ChangeOwnPassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	if err := security.ValidatePasswordPolicy(newPassword); err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}

	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return ErrUnauthorized
	}

	ok, err := security.VerifyPassword(user.PasswordHash, currentPassword)
	if err != nil {
		return ErrUnauthorized
	}
	if !ok {
		return ErrPasswordIncorrect
	}

	hash, err := security.HashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := s.store.UpdatePassword(ctx, userID, hash, false); err != nil {
		return err
	}

	return nil
}

func (s *Service) AdminResetPassword(ctx context.Context, userID, newPassword string) error {
	if err := security.ValidatePasswordPolicy(newPassword); err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}

	hash, err := security.HashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := s.store.UpdatePassword(ctx, userID, hash, true); err != nil {
		return err
	}

	return nil
}

func (s *Service) UnlockUser(ctx context.Context, userID string) error {
	return s.store.UnlockUser(ctx, userID)
}

func (s *Service) ListUserSessions(ctx context.Context, userID string) ([]Session, error) {
	return s.store.ListSessionsByUserID(ctx, userID)
}

func (s *Service) DeleteUserSessions(ctx context.Context, userID string) error {
	return s.store.DeleteSessionsByUserID(ctx, userID)
}

func (s *Service) ListUsers(ctx context.Context, page, pageSize int) ([]User, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	total, err := s.store.CountUsers(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	users, err := s.store.ListUsers(ctx, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (s *Service) CreateUser(ctx context.Context, username, displayName, password string, roles []string) (User, error) {
	if err := ValidateRoles(roles); err != nil {
		return User{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if err := security.ValidatePasswordPolicy(password); err != nil {
		return User{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return User{}, err
	}

	created, err := s.store.CreateUser(ctx, username, hash, displayName)
	if err != nil {
		return User{}, err
	}

	if err := s.store.ReplaceUserRoles(ctx, created.ID, roles); err != nil {
		return User{}, err
	}

	return s.store.GetUserByID(ctx, created.ID)
}

func (s *Service) UpdateUser(ctx context.Context, userID, username, displayName, status string) (User, error) {
	if status != "" && status != "active" && status != "disabled" {
		return User{}, fmt.Errorf("%w: invalid status", ErrValidation)
	}
	return s.store.UpdateUser(ctx, userID, username, displayName, status)
}

func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	return s.store.DeleteUser(ctx, userID)
}

func (s *Service) GetUser(ctx context.Context, userID string) (User, error) {
	return s.store.GetUserByID(ctx, userID)
}

func (s *Service) UpdateUserRoles(ctx context.Context, userID string, roles []string) error {
	if err := ValidateRoles(roles); err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return s.store.ReplaceUserRoles(ctx, userID, roles)
}

func (s *Service) ListAuditLogs(ctx context.Context, page, pageSize int) ([]AuditLog, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	total, err := s.store.CountAuditLogs(ctx)
	if err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	logs, err := s.store.ListAuditLogs(ctx, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (s *Service) WriteAuditLog(ctx context.Context, actorID *string, action, resourceType string, resourceID *string, detail map[string]any) error {
	return s.store.WriteAuditLog(ctx, actorID, action, resourceType, resourceID, detail)
}

func IsSessionExpired(lastActive, now time.Time, timeout time.Duration) bool {
	return now.Sub(lastActive) > timeout
}

func ShouldForcePasswordChangeBlock(forcePasswordChange bool, method, path string) bool {
	if !forcePasswordChange {
		return false
	}
	return !(method == "PATCH" && path == "/api/me/password")
}

func ValidateRoles(roles []string) error {
	if len(roles) == 0 {
		return fmt.Errorf("at least one role is required")
	}
	for _, role := range roles {
		if !IsValidRole(role) {
			return fmt.Errorf("invalid role: %s", role)
		}
	}
	return nil
}

func IsValidRole(role string) bool {
	return slices.Contains([]string{RoleFacilityAdmin, RoleDispatch, RoleFleetManager, RoleAuditor}, role)
}

func HasAnyRole(userRoles []string, required []string) bool {
	for _, r := range userRoles {
		if slices.Contains(required, r) {
			return true
		}
	}
	return false
}

func DecodeAuditDetail(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}
