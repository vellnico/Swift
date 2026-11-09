// Package service holds the business logic that ties the store, xray and the API together.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

// EventFunc writes a journal entry and pushes it to the UI.
type EventFunc func(level, source, key string, params map[string]any)

// Xray turns database state into a running xray process.
type Xray struct {
	Store      *store.Store
	Runner     xray.Runner
	Sup        *xray.Supervisor
	API        xray.UserAPI
	Bin        string
	ConfigPath string
	Log        *slog.Logger
	Event      EventFunc
	// Certs returns the certificate for Hysteria2, or nil if none is configured yet.
	Certs func(ctx context.Context) (*xray.CertFiles, error)
	// Progress reports apply stages to the UI: checking, restarting, done, error.
	Progress func(stage string, detail string)

	mu sync.Mutex
}

func (x *Xray) event(level, key string, params map[string]any) {
	if x.Event != nil {
		x.Event(level, "xray", key, params)
	}
}

func (x *Xray) progress(stage, detail string) {
	if x.Progress != nil {
		x.Progress(stage, detail)
	}
}

// EnsureAPIPort returns the loopback port of xray's API, choosing a free one once.
func EnsureAPIPort(ctx context.Context, st *store.Store) (int, error) {
	if v, err := st.Setting(ctx, "xray_api_port"); err == nil {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			return p, nil
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("pick api port: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	if err := st.SetSetting(ctx, "xray_api_port", strconv.Itoa(port)); err != nil {
		return 0, err
	}
	return port, nil
}

// Input collects the generator input from the database.
func (x *Xray) Input(ctx context.Context) (xray.Input, error) {
	port, err := EnsureAPIPort(ctx, x.Store)
	if err != nil {
		return xray.Input{}, err
	}
	profiles, err := x.Store.Profiles(ctx)
	if err != nil {
		return xray.Input{}, err
	}
	rows, err := x.Store.Users(ctx)
	if err != nil {
		return xray.Input{}, err
	}
	users := make([]xray.User, 0, len(rows))
	for _, u := range rows {
		users = append(users, ToXrayUser(u))
	}
	in := xray.Input{APIPort: port, Profiles: profiles, Users: users}
	if x.Certs != nil {
		if in.Cert, err = x.Certs(ctx); err != nil {
			return in, fmt.Errorf("certificate: %w", err)
		}
	}
	return in, nil
}

func ToXrayUser(u store.User) xray.User {
	return xray.User{ID: u.ID, Name: u.Name, VlessUUID: u.VlessUUID, Hy2Password: u.Hy2Password, Enabled: u.Enabled}
}

// ConfigRejected means xray refused the generated config; the old one stays.
type ConfigRejected struct{ Detail string }

func (e *ConfigRejected) Error() string { return "xray rejected the new config: " + e.Detail }

// writeConfig generates, tests and atomically installs the config.
// It returns the previous config so the caller can roll back.
func (x *Xray) writeConfig(ctx context.Context) (previous []byte, changed bool, err error) {
	in, err := x.Input(ctx)
	if err != nil {
		return nil, false, err
	}
	body, err := xray.Generate(in)
	if err != nil {
		return nil, false, fmt.Errorf("generate config: %w", err)
	}
	previous, _ = os.ReadFile(x.ConfigPath)
	if string(previous) == string(body) {
		return previous, false, nil
	}
	if err := os.MkdirAll(filepath.Dir(x.ConfigPath), 0o700); err != nil {
		return nil, false, err
	}
	// xray picks the format from the extension, so keep .json at the end
	tmp := strings.TrimSuffix(x.ConfigPath, ".json") + ".next.json"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return nil, false, fmt.Errorf("write config: %w", err)
	}
	if err := x.Runner.Test(ctx, x.Bin, tmp); err != nil {
		os.Remove(tmp)
		var te *xray.TestError
		if errors.As(err, &te) {
			return nil, false, &ConfigRejected{Detail: te.Output}
		}
		return nil, false, err
	}
	if err := os.Rename(tmp, x.ConfigPath); err != nil {
		os.Remove(tmp)
		return nil, false, fmt.Errorf("install config: %w", err)
	}
	return previous, true, nil
}

// Prepare writes the config before the supervisor starts; the DB is the source of truth.
func (x *Xray) Prepare(ctx context.Context) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	_, _, err := x.writeConfig(ctx)
	if err != nil {
		x.event("error", "xray.config_rejected", map[string]any{"error": err.Error()})
	}
	return err
}

// Apply regenerates the config, restarts xray and rolls back if it does not come up.
func (x *Xray) Apply(ctx context.Context) error { return x.apply(ctx, false) }

// Reload is Apply that restarts xray even when the config text is unchanged,
// e.g. after the certificate files were replaced in place.
func (x *Xray) Reload(ctx context.Context) error { return x.apply(ctx, true) }

func (x *Xray) apply(ctx context.Context, force bool) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.progress("checking", "")
	previous, changed, err := x.writeConfig(ctx)
	if err != nil {
		x.event("error", "xray.config_rejected", map[string]any{"error": err.Error()})
		x.progress("error", err.Error())
		return err
	}
	if !force && !changed && x.Sup.Status().Status == xray.StatusRunning {
		x.progress("done", "")
		return nil
	}
	x.progress("restarting", "")
	if err := x.restartAndWait(ctx); err != nil {
		x.Log.Error("xray did not come up, rolling back", "err", err)
		if previous != nil {
			_ = os.WriteFile(x.ConfigPath, previous, 0o600)
			_ = x.restartAndWait(ctx)
		}
		x.event("error", "xray.apply_failed", map[string]any{"error": err.Error()})
		x.progress("error", err.Error())
		return fmt.Errorf("apply config: %w", err)
	}
	x.event("info", "xray.config_applied", nil)
	x.progress("done", "")
	return nil
}

func (x *Xray) restartAndWait(ctx context.Context) error {
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := x.Sup.Restart(rctx); err != nil {
		return err
	}
	return x.WaitAPI(rctx)
}

// WaitAPI polls the gRPC API until xray answers or ctx ends.
func (x *Xray) WaitAPI(ctx context.Context) error {
	if x.API == nil {
		return nil
	}
	var last error
	for {
		pctx, cancel := context.WithTimeout(ctx, time.Second)
		last = x.API.Ping(pctx)
		cancel()
		if last == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("xray api not ready: %w", last)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// AddUser adds the user to every enabled inbound without restarting xray.
func (x *Xray) AddUser(ctx context.Context, u store.User) error {
	return x.eachInbound(ctx, func(p xray.Profile) error {
		return x.API.AddUser(ctx, p.Tag(), p.Kind, ToXrayUser(u))
	})
}

// RemoveUser removes the user from every enabled inbound without restarting xray.
func (x *Xray) RemoveUser(ctx context.Context, u store.User) error {
	return x.eachInbound(ctx, func(p xray.Profile) error {
		return x.API.RemoveUser(ctx, p.Tag(), ToXrayUser(u).Email())
	})
}

func (x *Xray) eachInbound(ctx context.Context, fn func(xray.Profile) error) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.Sup.Status().Status != xray.StatusRunning {
		// the config is regenerated from the DB on the next start anyway
		_, _, err := x.writeConfig(ctx)
		return err
	}
	profiles, err := x.Store.Profiles(ctx)
	if err != nil {
		return err
	}
	for _, p := range profiles {
		if !p.Enabled {
			continue
		}
		if err := fn(p); err != nil {
			return err
		}
	}
	// keep the file in sync so a later restart has the same users
	_, _, err = x.writeConfig(ctx)
	return err
}
