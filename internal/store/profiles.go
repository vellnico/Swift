package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vellnico/swift/internal/xray"
)

const profileCols = `id, name, kind, port, enabled, manual, params_json, position`

func scanProfile(sc interface{ Scan(...any) error }) (xray.Profile, error) {
	var p xray.Profile
	var kind, params string
	if err := sc.Scan(&p.ID, &p.Name, &kind, &p.Port, &p.Enabled, &p.Manual, &params, &p.Position); err != nil {
		return p, err
	}
	p.Kind = xray.Kind(kind)
	if err := json.Unmarshal([]byte(params), &p.Params); err != nil {
		return p, fmt.Errorf("profile %d params: %w", p.ID, err)
	}
	return p, nil
}

func (s *Store) Profiles(ctx context.Context) ([]xray.Profile, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+profileCols+` FROM profiles ORDER BY position, id`)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	defer rows.Close()
	out := []xray.Profile{}
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) Profile(ctx context.Context, id int64) (xray.Profile, error) {
	p, err := scanProfile(s.DB.QueryRowContext(ctx, `SELECT `+profileCols+` FROM profiles WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, fmt.Errorf("read profile: %w", err)
	}
	return p, nil
}

func (s *Store) CreateProfile(ctx context.Context, p xray.Profile) (xray.Profile, error) {
	params, err := json.Marshal(p.Params)
	if err != nil {
		return p, err
	}
	now := s.Now().Unix()
	err = s.Tx(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position) + 1, 0) FROM profiles`).Scan(&p.Position); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO profiles(name, kind, port, enabled, manual, params_json, position, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, p.Name, string(p.Kind), p.Port, p.Enabled, p.Manual, string(params), p.Position, now, now)
		if err != nil {
			return err
		}
		p.ID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return p, fmt.Errorf("create profile: %w", err)
	}
	return p, nil
}

func (s *Store) UpdateProfile(ctx context.Context, p xray.Profile) error {
	params, err := json.Marshal(p.Params)
	if err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE profiles SET name = ?, port = ?, enabled = ?, manual = ?, params_json = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.Port, p.Enabled, p.Manual, string(params), s.Now().Unix(), p.ID)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteProfile(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
