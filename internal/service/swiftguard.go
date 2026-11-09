package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/vellnico/swift/internal/store"
)

// An update is confirmed by the new version itself once it has run without
// trouble for a while; until then a guard started from the previous binary
// watches it and rolls everything back (SPEC 11.5).
const (
	swiftPendingKey    = "swift_update_pending"
	swiftConfirmedKey  = "swift_update_confirmed"
	swiftRolledBackKey = "swift_rolled_back"
	swiftSkipKey       = "swift_skip_version"
)

// PendingUpdate is what the guard needs to undo an update.
type PendingUpdate struct {
	From        string    `json:"from"`
	To          string    `json:"to"`
	Backup      string    `json:"backup"`      // database copy taken before the update
	XrayChanged bool      `json:"xrayChanged"` // xray.bak holds the previous Xray
	At          time.Time `json:"at"`
}

// RollbackInfo is shown once by the restored version.
type RollbackInfo struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"` // crash_loop, not_confirmed
}

func readJSONSetting(ctx context.Context, st *store.Store, key string, v any) bool {
	s, err := st.Setting(ctx, key)
	if err != nil || s == "" {
		return false
	}
	return json.Unmarshal([]byte(s), v) == nil
}

func writeJSONSetting(ctx context.Context, st *store.Store, key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return st.SetSetting(ctx, key, string(b))
}

// Systemd runs the systemctl and systemd-run commands the guard needs; mockable.
type Systemd interface {
	NRestarts(ctx context.Context, unit string) (int, error)
	Stop(ctx context.Context, unit string) error
	Start(ctx context.Context, unit string) error
	// RunTransient starts argv as its own unit, outside the panel's cgroup, so
	// it survives the panel restart.
	RunTransient(ctx context.Context, unit string, argv ...string) error
}

// ExecSystemd calls the real systemctl and systemd-run.
type ExecSystemd struct{}

func runCmd(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (ExecSystemd) NRestarts(ctx context.Context, unit string) (int, error) {
	out, err := runCmd(ctx, "systemctl", "show", "-p", "NRestarts", "--value", unit)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

func (ExecSystemd) Stop(ctx context.Context, unit string) error {
	_, err := runCmd(ctx, "systemctl", "stop", unit)
	return err
}

func (ExecSystemd) Start(ctx context.Context, unit string) error {
	_, err := runCmd(ctx, "systemctl", "start", unit)
	return err
}

func (ExecSystemd) RunTransient(ctx context.Context, unit string, argv ...string) error {
	_, err := runCmd(ctx, "systemd-run", append([]string{"--unit", unit, "--collect", "--quiet"}, argv...)...)
	return err
}

// UpdateGuard runs from the previous binary (`swift update-guard`) while the
// new version starts. It rolls back when the panel restarts in a loop or does
// not confirm itself in time.
type UpdateGuard struct {
	DBPath  string
	Binary  string // installed swift; the previous one is Binary + ".bak"
	XrayBin string // installed xray; the previous one is XrayBin + ".bak"
	Unit    string // systemd unit of the panel
	Systemd Systemd
	Log     io.Writer

	Timeout     time.Duration // the new version must confirm within this
	Interval    time.Duration
	MaxRestarts int
}

func (g *UpdateGuard) logf(format string, args ...any) {
	if g.Log != nil {
		fmt.Fprintf(g.Log, format+"\n", args...)
	}
}

func (g *UpdateGuard) withStore(ctx context.Context, fn func(*store.Store) error) error {
	st, err := store.Open(ctx, g.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()
	return fn(st)
}

// Run watches a pending update until it is confirmed or rolled back.
func (g *UpdateGuard) Run(ctx context.Context) error {
	var p PendingUpdate
	var found bool
	if err := g.withStore(ctx, func(st *store.Store) error {
		found = readJSONSetting(ctx, st, swiftPendingKey, &p)
		return nil
	}); err != nil {
		return err
	}
	if !found {
		g.logf("no pending update")
		return nil
	}
	g.logf("watching the update %s -> %s", p.From, p.To)
	base, baseErr := g.Systemd.NRestarts(ctx, g.Unit)
	deadline := time.Now().Add(g.Timeout)
	for {
		var confirmed string
		_ = g.withStore(ctx, func(st *store.Store) error {
			confirmed, _ = st.Setting(ctx, swiftConfirmedKey)
			return nil
		})
		if confirmed == p.To {
			g.logf("%s confirmed itself", p.To)
			return g.withStore(ctx, func(st *store.Store) error { return st.SetSetting(ctx, swiftPendingKey, "") })
		}
		if n, err := g.Systemd.NRestarts(ctx, g.Unit); baseErr == nil && err == nil && n-base >= g.MaxRestarts {
			return g.rollback(ctx, p, "crash_loop")
		}
		if time.Now().After(deadline) {
			return g.rollback(ctx, p, "not_confirmed")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(g.Interval):
		}
	}
}

func swapBack(path string) error {
	if _, err := os.Stat(path + ".bak"); err != nil {
		return fmt.Errorf("no backup of %s: %w", path, err)
	}
	_ = os.Remove(path + ".failed")
	if err := os.Rename(path, path+".failed"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(path+".bak", path)
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp := dst + ".restore"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func (g *UpdateGuard) rollback(ctx context.Context, p PendingUpdate, reason string) error {
	g.logf("rolling back %s -> %s: %s", p.To, p.From, reason)
	ctx = context.WithoutCancel(ctx)
	if err := g.Systemd.Stop(ctx, g.Unit); err != nil {
		g.logf("stop: %v", err)
	}
	var errs []error
	if err := swapBack(g.Binary); err != nil {
		errs = append(errs, fmt.Errorf("restore swift: %w", err))
	}
	if p.XrayChanged && g.XrayBin != "" {
		if err := swapBack(g.XrayBin); err != nil {
			errs = append(errs, fmt.Errorf("restore xray: %w", err))
		}
	}
	// the new version may have migrated the database; the copy taken before
	// the update matches the restored binary
	if p.Backup != "" {
		for _, suffix := range []string{"-wal", "-shm"} {
			_ = os.Remove(g.DBPath + suffix)
		}
		if err := copyFile(p.Backup, g.DBPath); err != nil {
			errs = append(errs, fmt.Errorf("restore database: %w", err))
		}
	}
	if err := g.withStore(ctx, func(st *store.Store) error {
		if err := writeJSONSetting(ctx, st, swiftRolledBackKey, RollbackInfo{From: p.From, To: p.To, Reason: reason}); err != nil {
			return err
		}
		if err := st.SetSetting(ctx, swiftSkipKey, p.To); err != nil {
			return err
		}
		return st.SetSetting(ctx, swiftPendingKey, "")
	}); err != nil {
		errs = append(errs, err)
	}
	if err := g.Systemd.Start(ctx, g.Unit); err != nil {
		errs = append(errs, fmt.Errorf("start: %w", err))
	}
	return errors.Join(errs...)
}
