package auth

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) GetUserByUsername(ctx context.Context, username string) (User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, organization_id, username, password_hash, COALESCE(display_name, ''), status,
			failed_login_count, locked_until, force_password_change, created_at
		FROM users
		WHERE username = $1
	`, username)

	var u User
	var id uuid.UUID
	var orgID *uuid.UUID
	if err := row.Scan(&id, &orgID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Status, &u.FailedLoginCount, &u.LockedUntil, &u.ForcePasswordChange, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	u.ID = id.String()
	if orgID != nil {
		v := orgID.String()
		u.OrganizationID = &v
	}

	roles, err := s.getUserRoles(ctx, id)
	if err != nil {
		return User{}, err
	}
	u.Roles = roles

	return u, nil
}

func (s *PostgresStore) GetUserByID(ctx context.Context, userID string) (User, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return User{}, ErrNotFound
	}

	row := s.pool.QueryRow(ctx, `
		SELECT id, organization_id, username, password_hash, COALESCE(display_name, ''), status,
			failed_login_count, locked_until, force_password_change, created_at
		FROM users
		WHERE id = $1
	`, uid)

	var u User
	var id uuid.UUID
	var orgID *uuid.UUID
	if err := row.Scan(&id, &orgID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Status, &u.FailedLoginCount, &u.LockedUntil, &u.ForcePasswordChange, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	u.ID = id.String()
	if orgID != nil {
		v := orgID.String()
		u.OrganizationID = &v
	}

	roles, err := s.getUserRoles(ctx, id)
	if err != nil {
		return User{}, err
	}
	u.Roles = roles

	return u, nil
}

func (s *PostgresStore) IncrementFailedLogin(ctx context.Context, userID string) (int, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return 0, ErrNotFound
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE users
		SET failed_login_count = failed_login_count + 1,
			updated_at = now()
		WHERE id = $1
		RETURNING failed_login_count
	`, uid)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (s *PostgresStore) SetLockedUntil(ctx context.Context, userID string, lockedUntil time.Time) (time.Time, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return time.Time{}, ErrNotFound
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE users
		SET locked_until = $2,
			updated_at = now()
		WHERE id = $1
		RETURNING locked_until
	`, uid, lockedUntil)

	var stored time.Time
	if err := row.Scan(&stored); err != nil {
		return time.Time{}, err
	}
	return stored, nil
}

func (s *PostgresStore) ClearLoginFailures(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrNotFound
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE users
		SET failed_login_count = 0,
			locked_until = NULL,
			updated_at = now()
		WHERE id = $1
	`, uid)
	return err
}

func (s *PostgresStore) CreateSession(ctx context.Context, userID string, now, expiresAt time.Time) (Session, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return Session{}, ErrNotFound
	}

	sid := uuid.New()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO sessions(id, user_id, created_at, last_active_at, expires_at)
		VALUES ($1, $2, $3, $3, $4)
	`, sid, uid, now, expiresAt)
	if err != nil {
		return Session{}, err
	}

	return Session{
		ID:           sid.String(),
		UserID:       uid.String(),
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *PostgresStore) GetSessionByID(ctx context.Context, sessionID string) (Session, error) {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return Session{}, ErrNotFound
	}

	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, created_at, last_active_at, expires_at
		FROM sessions
		WHERE id = $1
	`, sid)

	var session Session
	var id uuid.UUID
	var uid uuid.UUID
	if err := row.Scan(&id, &uid, &session.CreatedAt, &session.LastActiveAt, &session.ExpiresAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}

	session.ID = id.String()
	session.UserID = uid.String()
	return session, nil
}

func (s *PostgresStore) TouchSession(ctx context.Context, sessionID string, lastActive, expiresAt time.Time) error {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return ErrNotFound
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE sessions
		SET last_active_at = $2,
			expires_at = $3
		WHERE id = $1
	`, sid, lastActive, expiresAt)
	return err
}

func (s *PostgresStore) DeleteSession(ctx context.Context, sessionID string) error {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sid)
	return err
}

func (s *PostgresStore) DeleteSessionsByUserID(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, uid)
	return err
}

func (s *PostgresStore) ListSessionsByUserID(ctx context.Context, userID string) ([]Session, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrNotFound
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, created_at, last_active_at, expires_at
		FROM sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Session, 0)
	for rows.Next() {
		var session Session
		var sid uuid.UUID
		var suid uuid.UUID
		if err := rows.Scan(&sid, &suid, &session.CreatedAt, &session.LastActiveAt, &session.ExpiresAt); err != nil {
			return nil, err
		}
		session.ID = sid.String()
		session.UserID = suid.String()
		out = append(out, session)
	}

	return out, rows.Err()
}

func (s *PostgresStore) UpdatePassword(ctx context.Context, userID, passwordHash string, forcePasswordChange bool) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrNotFound
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE users
		SET password_hash = $2,
			force_password_change = $3,
			failed_login_count = 0,
			locked_until = NULL,
			updated_at = now()
		WHERE id = $1
	`, uid, passwordHash, forcePasswordChange)
	return err
}

func (s *PostgresStore) UnlockUser(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrNotFound
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE users
		SET failed_login_count = 0,
			locked_until = NULL,
			status = 'active',
			updated_at = now()
		WHERE id = $1
	`, uid)
	return err
}

func (s *PostgresStore) getUserRoles(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.name
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY r.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]string, 0)
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (s *PostgresStore) ListUsers(ctx context.Context, offset, limit int) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, organization_id, username, password_hash, COALESCE(display_name, ''), status,
			failed_login_count, locked_until, force_password_change, created_at
		FROM users
		ORDER BY created_at DESC
		OFFSET $1 LIMIT $2
	`, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var u User
		var id uuid.UUID
		var orgID *uuid.UUID
		if err := rows.Scan(&id, &orgID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Status, &u.FailedLoginCount, &u.LockedUntil, &u.ForcePasswordChange, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.ID = id.String()
		if orgID != nil {
			v := orgID.String()
			u.OrganizationID = &v
		}
		u.Roles, err = s.getUserRoles(ctx, id)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

func (s *PostgresStore) CountUsers(ctx context.Context) (int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *PostgresStore) CreateUser(ctx context.Context, username, passwordHash, displayName string) (User, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO users(organization_id, username, password_hash, display_name, status)
		VALUES ((SELECT id FROM organizations ORDER BY created_at LIMIT 1), $1, $2, $3, 'active')
		RETURNING id, organization_id, username, password_hash, COALESCE(display_name, ''), status,
			failed_login_count, locked_until, force_password_change, created_at
	`, username, passwordHash, displayName)

	var u User
	var id uuid.UUID
	var orgID *uuid.UUID
	if err := row.Scan(&id, &orgID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Status, &u.FailedLoginCount, &u.LockedUntil, &u.ForcePasswordChange, &u.CreatedAt); err != nil {
		return User{}, err
	}
	u.ID = id.String()
	if orgID != nil {
		v := orgID.String()
		u.OrganizationID = &v
	}
	u.Roles = []string{}
	return u, nil
}

func (s *PostgresStore) UpdateUser(ctx context.Context, userID, username, displayName, status string) (User, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return User{}, ErrNotFound
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE users
		SET username = CASE WHEN $2 = '' THEN username ELSE $2 END,
			display_name = CASE WHEN $3 = '' THEN display_name ELSE $3 END,
			status = CASE WHEN $4 = '' THEN status ELSE $4 END,
			updated_at = now()
		WHERE id = $1
		RETURNING id, organization_id, username, password_hash, COALESCE(display_name, ''), status,
			failed_login_count, locked_until, force_password_change, created_at
	`, uid, username, displayName, status)

	var u User
	var id uuid.UUID
	var orgID *uuid.UUID
	if err := row.Scan(&id, &orgID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Status, &u.FailedLoginCount, &u.LockedUntil, &u.ForcePasswordChange, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	u.ID = id.String()
	if orgID != nil {
		v := orgID.String()
		u.OrganizationID = &v
	}
	u.Roles, err = s.getUserRoles(ctx, id)
	if err != nil {
		return User{}, err
	}

	return u, nil
}

func (s *PostgresStore) DeleteUser(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid)
	return err
}

func (s *PostgresStore) ReplaceUserRoles(ctx context.Context, userID string, roles []string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrNotFound
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, uid); err != nil {
		return err
	}

	for _, role := range roles {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_roles(user_id, role_id)
			SELECT $1, id FROM roles WHERE name = $2
		`, uid, role); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) ListAuditLogs(ctx context.Context, offset, limit int) ([]AuditLog, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, actor_id, action, COALESCE(resource_type, ''), resource_id, COALESCE(detail, '{}'::jsonb), created_at
		FROM audit_logs
		ORDER BY created_at DESC
		OFFSET $1 LIMIT $2
	`, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]AuditLog, 0)
	for rows.Next() {
		var log AuditLog
		var id uuid.UUID
		var actorID *uuid.UUID
		var resourceID *uuid.UUID
		if err := rows.Scan(&id, &actorID, &log.Action, &log.ResourceType, &resourceID, &log.Detail, &log.CreatedAt); err != nil {
			return nil, err
		}
		log.ID = id.String()
		if actorID != nil {
			v := actorID.String()
			log.ActorID = &v
		}
		if resourceID != nil {
			v := resourceID.String()
			log.ResourceID = &v
		}
		logs = append(logs, log)
	}

	return logs, rows.Err()
}

func (s *PostgresStore) CountAuditLogs(ctx context.Context) (int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs`).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *PostgresStore) WriteAuditLog(ctx context.Context, actorID *string, action, resourceType string, resourceID *string, detail map[string]any) error {
	var actorUUID *uuid.UUID
	if actorID != nil {
		parsed, err := uuid.Parse(*actorID)
		if err == nil {
			actorUUID = &parsed
		}
	}

	var resourceUUID *uuid.UUID
	if resourceID != nil {
		parsed, err := uuid.Parse(*resourceID)
		if err == nil {
			resourceUUID = &parsed
		}
	}

	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_logs(actor_id, action, resource_type, resource_id, detail)
		VALUES ($1, $2, $3, $4, $5)
	`, actorUUID, action, resourceType, resourceUUID, detailJSON)
	return err
}
