package service

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/vellnico/swift/internal/store"
)

// guardEnv is a server right after an update to 0.4.0: new binaries in place,
// previous ones in .bak, a database backup from before the update.
func guardEnv(t *testing.T, sd Systemd) (*UpdateGuard, string) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	for name, body := range map[string]string{"swift": "new", "swift.bak": "old", "xray": "new-xray", "xray.bak": "old-xray"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	db := filepath.Join(dir, "swift.db")
	st, err := store.Open(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	_ = st.SetSetting(ctx, "marker", "before")
	backup := filepath.Join(dir, "backup.db")
	if err := st.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	_ = st.SetSetting(ctx, "marker", "after")
	if err := writeJSONSetting(ctx, st, swiftPendingKey, PendingUpdate{From: "0.3.0", To: "0.4.0", Backup: backup, XrayChanged: true}); err != nil {
		t.Fatal(err)
	}
	st.Close()
	g := &UpdateGuard{
		DBPath: db, Binary: filepath.Join(dir, "swift"), XrayBin: filepath.Join(dir, "xray"), Unit: "swift", Systemd: sd,
		Timeout: 2 * time.Second, Interval: 10 * time.Millisecond, MaxRestarts: 3,
	}
	return g, dir
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, _ := os.ReadFile(path)
	return string(b)
}

func setting(t *testing.T, g *UpdateGuard, key string) string {
	t.Helper()
	var v string
	_ = g.withStore(context.Background(), func(st *store.Store) error {
		v, _ = st.Setting(context.Background(), key)
		return nil
	})
	return v
}

func TestGuardKeepsAConfirmedUpdate(t *testing.T) {
	sd := &fakeSystemd{}
	g, dir := guardEnv(t, sd)
	_ = g.withStore(context.Background(), func(st *store.Store) error {
		return st.SetSetting(context.Background(), swiftConfirmedKey, "0.4.0")
	})
	if err := g.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "swift")) != "new" || len(sd.calls) != 0 || setting(t, g, swiftPendingKey) != "" {
		t.Fatalf("confirmed update touched: %v", sd.calls)
	}
}

func TestGuardRollsBackACrashLoop(t *testing.T) {
	sd := &fakeSystemd{grow: true}
	g, dir := guardEnv(t, sd)
	start := time.Now()
	if err := g.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("a crash loop must not wait for the timeout")
	}
	if read(t, filepath.Join(dir, "swift")) != "old" || read(t, filepath.Join(dir, "swift.failed")) != "new" {
		t.Fatal("swift binary not restored")
	}
	if read(t, filepath.Join(dir, "xray")) != "old-xray" {
		t.Fatal("xray binary not restored")
	}
	if setting(t, g, "marker") != "before" {
		t.Fatal("database not restored from the backup")
	}
	var rb RollbackInfo
	_ = g.withStore(context.Background(), func(st *store.Store) error {
		readJSONSetting(context.Background(), st, swiftRolledBackKey, &rb)
		return nil
	})
	if rb.Reason != "crash_loop" || rb.To != "0.4.0" || setting(t, g, swiftSkipKey) != "0.4.0" || setting(t, g, swiftPendingKey) != "" {
		t.Fatalf("rollback record %+v", rb)
	}
	if !slices.Equal(sd.calls, []string{"stop swift", "start swift"}) {
		t.Fatalf("systemd calls %v", sd.calls)
	}

	// the restored version reports the rollback once
	var events []string
	u := &SwiftUpdater{Store: nil, Version: "v0.3.0", GOOS: "linux", Event: func(_, _, key string, _ map[string]any) { events = append(events, key) }}
	_ = g.withStore(context.Background(), func(st *store.Store) error {
		u.Store = st
		u.ReportStartup(context.Background())
		u.ReportStartup(context.Background())
		if info := u.Info(context.Background()); info.Skipped != "0.4.0" {
			t.Errorf("skipped %q", info.Skipped)
		}
		return nil
	})
	if !slices.Equal(events, []string{"swift.rolled_back"}) {
		t.Fatalf("events %v", events)
	}
}

func TestGuardRollsBackWithoutConfirmation(t *testing.T) {
	sd := &fakeSystemd{}
	g, dir := guardEnv(t, sd)
	g.Timeout = 100 * time.Millisecond
	if err := g.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "swift")) != "old" {
		t.Fatal("not rolled back")
	}
	var rb RollbackInfo
	_ = g.withStore(context.Background(), func(st *store.Store) error {
		readJSONSetting(context.Background(), st, swiftRolledBackKey, &rb)
		return nil
	})
	if rb.Reason != "not_confirmed" {
		t.Fatalf("reason %q", rb.Reason)
	}
}

func TestAutoUpdateSkipsARolledBackRelease(t *testing.T) {
	ctx := context.Background()
	u, _, _, _ := swiftEnv(t)
	_ = u.SetAuto(ctx, true)
	_ = u.Store.SetSetting(ctx, swiftSkipKey, "0.4.0")
	u.nightly(ctx, true)
	if read(t, u.Binary) != "old" {
		t.Fatal("auto-update installed a release that was rolled back")
	}
}

func TestGuardRollsBackAFailedUnit(t *testing.T) {
	sd := &fakeSystemd{state: "failed"}
	g, dir := guardEnv(t, sd)
	if err := g.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "swift")) != "old" {
		t.Fatal("a unit systemd gave up on was not rolled back")
	}
}
