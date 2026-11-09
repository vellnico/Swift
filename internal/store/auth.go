package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Admin struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type Session struct {
	ID        string
	AdminID   int64
	ExpiresAt time.Time
	IP        string
	UserAgent string
}

func scanAdmin(row *sql.Row) (Admin, error) {
	var a Admin
	var created int64
	err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, fmt.Errorf("read admin: %w", err)
	}
	a.CreatedAt = time.Unix(created, 0)
	return a, nil
}

func (s *Store) AdminByUsername(ctx context.Context, username string) (Admin, error) {
	return scanAdmin(s.DB.QueryRowContext(ctx, `SELECT id, username, password_hash, created_at FROM admins WHERE username = ?`, username))
}

func (s *Store) AdminByID(ctx context.Context, id int64) (Admin, error) {
	return scanAdmin(s.DB.QueryRowContext(ctx, `SELECT id, username, password_hash, created_at FROM admins WHERE id = ?`, id))
}

// FirstAdmin returns the single panel administrator, if any.
func (s *Store) FirstAdmin(ctx context.Context) (Admin, error) {
	return scanAdmin(s.DB.QueryRowContext(ctx, `SELECT id, username, password_hash, created_at FROM admins ORDER BY id LIMIT 1`))
}

func (s *Store) CreateAdmin(ctx context.Context, username, hash string) (Admin, error) {
	now := s.Now()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO admins(username, password_hash, created_at) VALUES (?, ?, ?)`, username, hash, now.Unix())
	if err != nil {
		return Admin{}, fmt.Errorf("create admin: %w", err)
	}
	id, _ := res.LastInsertId()
	return Admin{ID: id, Username: username, PasswordHash: hash, CreatedAt: now}, nil
}

// SetAdminPassword replaces the hash and drops every session of that admin.
func (s *Store) SetAdminPassword(ctx context.Context, id int64, hash string) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE admins SET password_hash = ? WHERE id = ?`, hash, id); err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE admin_id = ?`, id); err != nil {
			return fmt.Errorf("drop sessions: %w", err)
		}
		return nil
	})
}

func (s *Store) CreateSession(ctx context.Context, ses Session) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO sessions(id, admin_id, expires_at, ip, user_agent) VALUES (?, ?, ?, ?, ?)`,
		ses.ID, ses.AdminID, ses.ExpiresAt.Unix(), ses.IP, ses.UserAgent)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *Store) SessionByID(ctx context.Context, id string) (Session, error) {
	var ses Session
	var exp int64
	var ip, ua sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT id, admin_id, expires_at, ip, user_agent FROM sessions WHERE id = ?`, id).
		Scan(&ses.ID, &ses.AdminID, &exp, &ip, &ua)
	if errors.Is(err, sql.ErrNoRows) {
		return ses, ErrNotFound
	}
	if err != nil {
		return ses, fmt.Errorf("read session: %w", err)
	}
	ses.ExpiresAt, ses.IP, ses.UserAgent = time.Unix(exp, 0), ip.String, ua.String
	return ses, nil
}

func (s *Store) ExtendSession(ctx context.Context, id string, expires time.Time) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE sessions SET expires_at = ? WHERE id = ?`, expires.Unix(), id)
	if err != nil {
		return fmt.Errorf("extend session: %w", err)
	}
	return nil
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, s.Now().Unix())
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}
