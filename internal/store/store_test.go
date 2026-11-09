package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// OpenTest opens a fresh database in a temp dir for tests in other packages too.
func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "swift.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestMigrationsApplyAndAreIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "swift.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"admins", "sessions", "settings", "profiles", "users", "certificates", "events"} {
		var name string
		if err := s.DB.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Errorf("table %s missing: %v", table, err)
		}
	}
	var mode string
	if err := s.DB.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
		t.Errorf("journal_mode = %q, %v", mode, err)
	}
	var fk int
	if err := s.DB.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Errorf("foreign_keys = %d, %v", fk, err)
	}
	s.Close()

	s, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s.Close()
	var n int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("schema_migrations rows = %d, %v", n, err)
	}
}

func TestProfileKindCheck(t *testing.T) {
	s := openTest(t)
	_, err := s.DB.Exec(`INSERT INTO profiles(name, kind, port, params_json, position, created_at, updated_at)
		VALUES ('x', 'wireguard', 1, '{}', 0, 0, 0)`)
	if err == nil {
		t.Fatal("expected CHECK constraint failure")
	}
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	if _, err := s.Setting(ctx, "panel_port"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := s.SetSetting(ctx, "panel_port", "1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(ctx, "panel_port", "2"); err != nil {
		t.Fatal(err)
	}
	v, err := s.Setting(ctx, "panel_port")
	if err != nil || v != "2" {
		t.Fatalf("got %q, %v", v, err)
	}
	all, err := s.Settings(ctx)
	if err != nil || all["panel_port"] != "2" {
		t.Fatalf("settings = %v, %v", all, err)
	}
}
