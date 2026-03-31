package unit_tests

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"parkops/internal/auth"
	"parkops/internal/platform/security"
)

type memoryAuthStore struct {
	users    map[string]auth.User
	byName   map[string]string
	sessions map[string]auth.Session
	audit    []auth.AuditLog
	nextSID  int
}

func newMemoryAuthStore(t *testing.T) *memoryAuthStore {
	t.Helper()
	hash, err := security.HashPassword("ValidPass123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u := auth.User{ID: "user-1", Username: "user", PasswordHash: hash, Roles: []string{"facility_admin"}}
	return &memoryAuthStore{
		users:    map[string]auth.User{u.ID: u},
		byName:   map[string]string{u.Username: u.ID},
		sessions: map[string]auth.Session{},
		nextSID:  1,
	}
}

func (m *memoryAuthStore) GetUserByUsername(_ context.Context, username string) (auth.User, error) {
	id, ok := m.byName[username]
	if !ok {
		return auth.User{}, auth.ErrNotFound
	}
	return m.users[id], nil
}
func (m *memoryAuthStore) GetUserByID(_ context.Context, userID string) (auth.User, error) {
	u, ok := m.users[userID]
	if !ok {
		return auth.User{}, auth.ErrNotFound
	}
	return u, nil
}
func (m *memoryAuthStore) IncrementFailedLogin(_ context.Context, userID string) (int, error) {
	u := m.users[userID]
	u.FailedLoginCount++
	m.users[userID] = u
	return u.FailedLoginCount, nil
}
func (m *memoryAuthStore) SetLockedUntil(_ context.Context, userID string, lockedUntil time.Time) (time.Time, error) {
	u := m.users[userID]
	u.LockedUntil = &lockedUntil
	m.users[userID] = u
	return lockedUntil, nil
}
func (m *memoryAuthStore) ClearLoginFailures(_ context.Context, userID string) error {
	u := m.users[userID]
	u.FailedLoginCount = 0
	u.LockedUntil = nil
	m.users[userID] = u
	return nil
}
func (m *memoryAuthStore) ListUsers(_ context.Context, offset, limit int) ([]auth.User, error) {
	out := make([]auth.User, 0, len(m.users))
	for _, u := range m.users {
		out = append(out, u)
	}
	if offset >= len(out) {
		return []auth.User{}, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
}
func (m *memoryAuthStore) CountUsers(_ context.Context) (int, error) {
	return len(m.users), nil
}
func (m *memoryAuthStore) CreateUser(_ context.Context, username, passwordHash, displayName string) (auth.User, error) {
	id := "user-created"
	u := auth.User{ID: id, Username: username, PasswordHash: passwordHash, DisplayName: displayName, Status: "active"}
	m.users[id] = u
	m.byName[username] = id
	return u, nil
}
func (m *memoryAuthStore) UpdateUser(_ context.Context, userID, username, displayName, status string) (auth.User, error) {
	u, ok := m.users[userID]
	if !ok {
		return auth.User{}, auth.ErrNotFound
	}
	if username != "" {
		u.Username = username
	}
	if displayName != "" {
		u.DisplayName = displayName
	}
	if status != "" {
		u.Status = status
	}
	m.users[userID] = u
	return u, nil
}
func (m *memoryAuthStore) DeleteUser(_ context.Context, userID string) error {
	delete(m.users, userID)
	return nil
}
func (m *memoryAuthStore) ReplaceUserRoles(_ context.Context, userID string, roles []string) error {
	u := m.users[userID]
	u.Roles = roles
	m.users[userID] = u
	return nil
}
func (m *memoryAuthStore) ListAuditLogs(_ context.Context, offset, limit int) ([]auth.AuditLog, error) {
	if offset >= len(m.audit) {
		return []auth.AuditLog{}, nil
	}
	end := offset + limit
	if end > len(m.audit) {
		end = len(m.audit)
	}
	return m.audit[offset:end], nil
}
func (m *memoryAuthStore) CountAuditLogs(_ context.Context) (int, error) {
	return len(m.audit), nil
}
func (m *memoryAuthStore) WriteAuditLog(_ context.Context, actorID *string, action, resourceType string, resourceID *string, detail map[string]any) error {
	b, _ := json.Marshal(detail)
	m.audit = append(m.audit, auth.AuditLog{Action: action, ResourceType: resourceType, ActorID: actorID, ResourceID: resourceID, Detail: b, CreatedAt: time.Now().UTC()})
	return nil
}
func (m *memoryAuthStore) CreateSession(_ context.Context, userID string, now, expiresAt time.Time) (auth.Session, error) {
	id := "s-" + time.Now().Format("150405")
	s := auth.Session{ID: id, UserID: userID, CreatedAt: now, LastActiveAt: now, ExpiresAt: expiresAt}
	m.sessions[id] = s
	return s, nil
}
func (m *memoryAuthStore) GetSessionByID(_ context.Context, sessionID string) (auth.Session, error) {
	s, ok := m.sessions[sessionID]
	if !ok {
		return auth.Session{}, auth.ErrNotFound
	}
	return s, nil
}
func (m *memoryAuthStore) TouchSession(_ context.Context, sessionID string, lastActive, expiresAt time.Time) error {
	s := m.sessions[sessionID]
	s.LastActiveAt = lastActive
	s.ExpiresAt = expiresAt
	m.sessions[sessionID] = s
	return nil
}
func (m *memoryAuthStore) DeleteSession(_ context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}
func (m *memoryAuthStore) DeleteSessionsByUserID(_ context.Context, userID string) error {
	for id, s := range m.sessions {
		if s.UserID == userID {
			delete(m.sessions, id)
		}
	}
	return nil
}
func (m *memoryAuthStore) ListSessionsByUserID(_ context.Context, userID string) ([]auth.Session, error) {
	out := make([]auth.Session, 0)
	for _, s := range m.sessions {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}
func (m *memoryAuthStore) UpdatePassword(_ context.Context, userID, passwordHash string, forcePasswordChange bool) error {
	u := m.users[userID]
	u.PasswordHash = passwordHash
	u.ForcePasswordChange = forcePasswordChange
	m.users[userID] = u
	return nil
}
func (m *memoryAuthStore) UnlockUser(_ context.Context, userID string) error {
	u := m.users[userID]
	u.FailedLoginCount = 0
	u.LockedUntil = nil
	m.users[userID] = u
	return nil
}

func TestLockoutAfterFiveFailures(t *testing.T) {
	store := newMemoryAuthStore(t)
	svc := auth.NewService(store)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return now })

	for i := 0; i < 4; i++ {
		_, err := svc.Login(context.Background(), "user", "wrong")
		if !errors.Is(err, auth.ErrUnauthorized) {
			t.Fatalf("attempt %d expected unauthorized, got %v", i+1, err)
		}
	}

	_, err := svc.Login(context.Background(), "user", "wrong")
	if !errors.Is(err, auth.ErrRateLimited) {
		t.Fatalf("fifth attempt expected rate limited, got %v", err)
	}
}

func TestSessionTimeout(t *testing.T) {
	store := newMemoryAuthStore(t)
	svc := auth.NewService(store)
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return start })

	result, err := svc.Login(context.Background(), "user", "ValidPass123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	svc.SetNowFunc(func() time.Time { return start.Add(auth.SessionTimeout + time.Second) })
	_, _, err = svc.AuthenticateSession(context.Background(), result.Session.ID)
	if !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for expired session, got %v", err)
	}
}

func TestForcePasswordChangeBlocksRoutes(t *testing.T) {
	if !auth.ShouldForcePasswordChangeBlock(true, "GET", "/api/me") {
		t.Fatal("expected force change to block /api/me")
	}
	if auth.ShouldForcePasswordChangeBlock(true, "PATCH", "/api/me/password") {
		t.Fatal("expected force change to allow /api/me/password")
	}
}
