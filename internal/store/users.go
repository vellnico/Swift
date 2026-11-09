package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type User struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	SubToken    string    `json:"-"`
	VlessUUID   string    `json:"-"`
	Hy2Password string    `json:"-"`
	Enabled     bool      `json:"enabled"`
	Note        string    `json:"note"`
	CreatedAt   time.Time `json:"createdAt"`
}

var ErrDuplicate = errors.New("duplicate")

const userCols = `id, name, sub_token, vless_uuid, hy2_password, enabled, note, created_at`

func scanUser(sc interface{ Scan(...any) error }) (User, error) {
	var u User
	var created int64
	err := sc.Scan(&u.ID, &u.Name, &u.SubToken, &u.VlessUUID, &u.Hy2Password, &u.Enabled, &u.Note, &created)
	u.CreatedAt = time.Unix(created, 0)
	return u, err
}

func (s *Store) Users(ctx context.Context) ([]User, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) userBy(ctx context.Context, col string, v any) (User, error) {
	u, err := scanUser(s.DB.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE `+col+` = ?`, v))
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	if err != nil {
		return u, fmt.Errorf("read user: %w", err)
	}
	return u, nil
}

func (s *Store) User(ctx context.Context, id int64) (User, error) { return s.userBy(ctx, "id", id) }

func (s *Store) UserByToken(ctx context.Context, token string) (User, error) {
	return s.userBy(ctx, "sub_token", token)
}

func (s *Store) CreateUser(ctx context.Context, u User) (User, error) {
	u.CreatedAt = s.Now()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO users(name, sub_token, vless_uuid, hy2_password, enabled, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.Name, u.SubToken, u.VlessUUID, u.Hy2Password, u.Enabled, u.Note, u.CreatedAt.Unix())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return u, ErrDuplicate
		}
		return u, fmt.Errorf("create user: %w", err)
	}
	u.ID, _ = res.LastInsertId()
	return u, nil
}

// RestoreUser re-inserts a deleted user with its original id (undo after delete).
func (s *Store) RestoreUser(ctx context.Context, u User) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO users(id, name, sub_token, vless_uuid, hy2_password, enabled, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Name, u.SubToken, u.VlessUUID, u.Hy2Password, u.Enabled, u.Note, u.CreatedAt.Unix())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ErrDuplicate
		}
		return fmt.Errorf("restore user: %w", err)
	}
	return nil
}

func (s *Store) UpdateUser(ctx context.Context, u User) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE users SET name = ?, sub_token = ?, vless_uuid = ?, hy2_password = ?, enabled = ?, note = ? WHERE id = ?`,
		u.Name, u.SubToken, u.VlessUUID, u.Hy2Password, u.Enabled, u.Note, u.ID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ErrDuplicate
		}
		return fmt.Errorf("update user: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
