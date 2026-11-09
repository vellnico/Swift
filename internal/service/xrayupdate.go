package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

// XrayUpdater replaces the xray binary from the panel (SPEC 11.4).
type XrayUpdater struct {
	Store  *store.Store
	Xray   *Xray
	Source xray.ReleaseSource
	Asset  string // release archive for this platform
	Binary string // executable name inside the archive
	// Probe reads the version of a binary (`xray version`); replaced in tests.
	Probe func(ctx context.Context, bin string) (string, error)
	// UpTimeout is how long the new binary has to answer on the API.
	UpTimeout time.Duration
	Event     EventFunc
	// Progress reports update stages: downloading, verifying, testing, restarting, done, rolled_back, error.
	Progress func(stage, detail string)
	// OnVersion is called when the installed version changes.
	OnVersion func(version string)

	mu      sync.Mutex
	running sync.Mutex
	current string
}

type XrayRelease struct {
	Current   string    `json:"current"`
	Pinned    string    `json:"pinned"`
	Latest    string    `json:"latest"`
	CheckedAt time.Time `json:"checkedAt"`
	Available bool      `json:"available"`
	Major     bool      `json:"major"`
	Updating  bool      `json:"updating"`
	Backup    bool      `json:"backup"`

	// BreaksHiddify: releases newer than the pinned one reject REALITY from sing-box clients.
	BreaksHiddify bool `json:"breaksHiddify"`
}

var ErrUpdateBusy = errors.New("an xray update is already running")

// UpdateError is a failed update the UI can explain; Stage is where it stopped.
type UpdateError struct {
	Code   string // download, checksum, rejected, rolled_back
	Detail string
}

func (e *UpdateError) Error() string { return "xray update " + e.Code + ": " + e.Detail }

func (u *XrayUpdater) event(level, key string, p map[string]any) {
	if u.Event != nil {
		u.Event(level, "xray", key, p)
	}
}

func (u *XrayUpdater) progress(stage, detail string) {
	if u.Progress != nil {
		u.Progress(stage, detail)
	}
}

// Current is the version of the installed binary, read once and cached.
func (u *XrayUpdater) Current(ctx context.Context) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.current == "" && u.Probe != nil {
		if v, err := u.Probe(ctx, u.Xray.Bin); err == nil {
			u.current = v
		}
	}
	if u.current == "" {
		return xray.Version
	}
	return u.current
}

func (u *XrayUpdater) setCurrent(v string) {
	u.mu.Lock()
	u.current = v
	u.mu.Unlock()
	if u.OnVersion != nil {
		u.OnVersion(v)
	}
}

// Info returns what Settings shows without touching the network.
func (u *XrayUpdater) Info(ctx context.Context) XrayRelease {
	r := XrayRelease{Current: u.Current(ctx), Pinned: xray.Version}
	r.Latest, _ = u.Store.Setting(ctx, "xray_latest")
	if v, err := u.Store.Setting(ctx, "xray_latest_checked"); err == nil {
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			r.CheckedAt = time.UnixMilli(ms)
		}
	}
	if r.Latest != "" {
		r.Available = xray.CompareVersions(r.Latest, r.Current) > 0
		r.Major = r.Available && xray.MajorChanged(r.Current, r.Latest)
		r.BreaksHiddify = r.Available && xray.CompareVersions(r.Latest, xray.Version) > 0
	}
	if u.running.TryLock() {
		u.running.Unlock()
	} else {
		r.Updating = true
	}
	if _, err := os.Stat(u.Xray.Bin + ".bak"); err == nil {
		r.Backup = true
	}
	return r
}

// Check asks the release source for the newest version and remembers it.
func (u *XrayUpdater) Check(ctx context.Context) (XrayRelease, error) {
	v, err := u.Source.Latest(ctx)
	if err != nil {
		return u.Info(ctx), fmt.Errorf("check xray release: %w", err)
	}
	if err := u.Store.SetSetting(ctx, "xray_latest", v); err != nil {
		return XrayRelease{}, err
	}
	if err := u.Store.SetSetting(ctx, "xray_latest_checked", strconv.FormatInt(u.Store.Now().UnixMilli(), 10)); err != nil {
		return XrayRelease{}, err
	}
	return u.Info(ctx), nil
}

// RunChecks checks for a new release once a day until ctx ends.
func (u *XrayUpdater) RunChecks(ctx context.Context, every time.Duration) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		info := u.Info(ctx)
		if u.Store.Now().Sub(info.CheckedAt) >= every {
			before := info.Latest
			if after, err := u.Check(ctx); err == nil && after.Available && after.Latest != before {
				u.event("info", "xray.update_available", map[string]any{"version": after.Latest})
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Update installs version: download, verify SHA256, `xray run -test` with the
// current config, swap the binary, restart; a binary that does not come up
// within UpTimeout is replaced by the previous one.
func (u *XrayUpdater) Update(ctx context.Context, version string) (err error) {
	if !xray.ValidVersion(version) {
		return &ValidationError{Fields: map[string]string{"version": "invalid"}}
	}
	if !u.running.TryLock() {
		return ErrUpdateBusy
	}
	defer u.running.Unlock()
	from := u.Current(ctx)
	defer func() {
		var ue *UpdateError
		switch {
		case err == nil:
		case errors.As(err, &ue) && ue.Code == "rolled_back":
			u.progress("rolled_back", ue.Detail)
		default:
			u.progress("error", err.Error())
			u.event("error", "xray.update_failed", map[string]any{"version": version, "error": err.Error()})
		}
	}()

	u.progress("downloading", version)
	archive, want, err := u.Source.Archive(ctx, version, u.Asset)
	if err != nil {
		return &UpdateError{Code: "download", Detail: err.Error()}
	}
	u.progress("verifying", "")
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return &UpdateError{Code: "checksum", Detail: fmt.Sprintf("sha256 %s, expected %s", got, want)}
	}
	body, err := xray.ExtractFile(archive, u.Binary)
	if err != nil {
		return &UpdateError{Code: "download", Detail: err.Error()}
	}
	bin := u.Xray.Bin
	next := bin + ".new"
	if err := os.WriteFile(next, body, 0o755); err != nil {
		return fmt.Errorf("write new xray: %w", err)
	}
	defer os.Remove(next)

	u.progress("testing", "")
	if err := u.Xray.Runner.Test(ctx, next, u.Xray.ConfigPath); err != nil {
		var te *xray.TestError
		if errors.As(err, &te) {
			return &UpdateError{Code: "rejected", Detail: te.Output}
		}
		return &UpdateError{Code: "rejected", Detail: err.Error()}
	}

	x := u.Xray
	x.mu.Lock()
	defer x.mu.Unlock()
	backup := bin + ".bak"
	_ = os.Remove(backup) // the previous backup lives until the next update
	if err := os.Rename(bin, backup); err != nil {
		return fmt.Errorf("back up xray: %w", err)
	}
	if err := os.Rename(next, bin); err != nil {
		_ = os.Rename(backup, bin)
		return fmt.Errorf("install new xray: %w", err)
	}

	u.progress("restarting", "")
	timeout := u.UpTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	if upErr := x.restartWithin(ctx, timeout); upErr != nil {
		x.Log.Error("new xray did not come up, restoring the previous binary", "version", version, "err", upErr)
		failed := bin + ".failed"
		_ = os.Remove(failed)
		if err := os.Rename(bin, failed); err != nil {
			return fmt.Errorf("roll back xray: %w", err)
		}
		if err := os.Rename(backup, bin); err != nil {
			return fmt.Errorf("roll back xray: %w", err)
		}
		_ = os.Remove(failed)
		if err := x.restartAndWait(ctx); err != nil {
			x.Log.Error("previous xray did not come up either", "err", err)
		}
		u.event("error", "xray.update_rolled_back", map[string]any{"version": version, "from": from})
		return &UpdateError{Code: "rolled_back", Detail: upErr.Error()}
	}
	u.setCurrent(version)
	u.event("info", "xray.updated", map[string]any{"from": from, "to": version})
	u.progress("done", version)
	return nil
}
