package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vellnico/swift/internal/selfupdate"
	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

// SwiftUpdater updates the panel itself: a nightly check, an update button and
// an optional nightly auto-update. The new binary is verified, the database is
// backed up, and systemd starts the new version after the process exits.
type SwiftUpdater struct {
	Store   *store.Store
	Source  selfupdate.Source
	Version string // running version, e.g. "v0.3.0"; "dev" disables updates
	Binary  string // path of the running executable
	DataDir string // where database backups go
	GOOS    string
	GOARCH  string
	// Xray brings Xray to the version the new release pins, unless the
	// administrator installed another version on purpose; nil skips it.
	Xray *XrayUpdater
	// Probe runs `<bin> version` and returns its output; replaced in tests.
	Probe func(ctx context.Context, bin string) (string, error)
	// Restart stops the process so systemd starts the new binary.
	Restart  func()
	Event    EventFunc
	Progress func(stage, detail string)

	running sync.Mutex
}

type SwiftRelease struct {
	Current   string    `json:"current"`
	Latest    string    `json:"latest"`
	CheckedAt time.Time `json:"checkedAt"`
	NextCheck time.Time `json:"nextCheck"`
	Available bool      `json:"available"`
	Updating  bool      `json:"updating"`
	Auto      bool      `json:"auto"`

	// Supported is false for development builds: they are updated from source.
	Supported bool `json:"supported"`
}

const (
	swiftLatestKey  = "swift_latest"
	swiftCheckedKey = "swift_latest_checked"
	swiftAutoKey    = "swift_auto_update"
	swiftFromKey    = "swift_updated_from"
)

// swiftCheckOffset runs the panel check half an hour after the Xray one.
const swiftCheckOffset = 30 * time.Minute

func (u *SwiftUpdater) event(level, key string, p map[string]any) {
	if u.Event != nil {
		u.Event(level, "swift", key, p)
	}
}

func (u *SwiftUpdater) progress(stage, detail string) {
	if u.Progress != nil {
		u.Progress(stage, detail)
	}
}

func (u *SwiftUpdater) current() string { return selfupdate.Normalize(u.Version) }

// Info returns what Settings shows without touching the network.
func (u *SwiftUpdater) Info(ctx context.Context) SwiftRelease {
	r := SwiftRelease{Current: u.current(), Supported: u.current() != "" && u.GOOS == "linux"}
	if r.Current == "" {
		r.Current = u.Version
	}
	r.Latest, _ = u.Store.Setting(ctx, swiftLatestKey)
	if v, err := u.Store.Setting(ctx, swiftCheckedKey); err == nil {
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			r.CheckedAt = time.UnixMilli(ms)
		}
	}
	r.NextCheck = NextNight(time.Now(), swiftCheckOffset)
	r.Available = r.Supported && r.Latest != "" && xray.CompareVersions(r.Latest, r.Current) > 0
	r.Auto, _ = u.autoEnabled(ctx)
	if u.running.TryLock() {
		u.running.Unlock()
	} else {
		r.Updating = true
	}
	return r
}

func (u *SwiftUpdater) autoEnabled(ctx context.Context) (bool, error) {
	v, err := u.Store.Setting(ctx, swiftAutoKey)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return v == "1", err
}

// SetAuto turns the nightly auto-update on or off.
func (u *SwiftUpdater) SetAuto(ctx context.Context, on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	return u.Store.SetSetting(ctx, swiftAutoKey, v)
}

// Check asks GitHub for the newest release and remembers it.
func (u *SwiftUpdater) Check(ctx context.Context) (SwiftRelease, error) {
	v, err := u.Source.Latest(ctx)
	if err != nil {
		return u.Info(ctx), fmt.Errorf("check swift release: %w", err)
	}
	if err := u.Store.SetSetting(ctx, swiftLatestKey, v); err != nil {
		return SwiftRelease{}, err
	}
	if err := u.Store.SetSetting(ctx, swiftCheckedKey, strconv.FormatInt(u.Store.Now().UnixMilli(), 10)); err != nil {
		return SwiftRelease{}, err
	}
	return u.Info(ctx), nil
}

// RunChecks checks every night and, with auto-update on, installs a new release.
func (u *SwiftUpdater) RunChecks(ctx context.Context) {
	runNightly(ctx, swiftCheckOffset, func() time.Time { return u.Info(ctx).CheckedAt }, func(night bool) {
		u.nightly(ctx, night)
	})
}

func (u *SwiftUpdater) nightly(ctx context.Context, night bool) {
	if !u.Info(ctx).Supported {
		return
	}
	before := u.Info(ctx).Latest
	after, err := u.Check(ctx)
	if err != nil || !after.Available {
		return
	}
	if after.Latest != before {
		u.event("info", "swift.update_available", map[string]any{"version": after.Latest})
	}
	if night && after.Auto {
		_ = u.Update(ctx, after.Latest)
	}
}

// ReportStartup journals a finished update once the new version runs.
func (u *SwiftUpdater) ReportStartup(ctx context.Context) {
	from, err := u.Store.Setting(ctx, swiftFromKey)
	if err != nil || from == "" {
		return
	}
	_ = u.Store.SetSetting(ctx, swiftFromKey, "")
	if from != u.current() {
		u.event("info", "swift.updated", map[string]any{"from": from, "to": u.current()})
	}
}

var swiftVersionOut = regexp.MustCompile(`^swift (\S+) xray (\S+)`)

func (u *SwiftUpdater) probe(ctx context.Context, bin string) (string, error) {
	if u.Probe != nil {
		return u.Probe(ctx, bin)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "version").Output()
	return string(out), err
}

// Update installs version: download, verify SHA256, run the new binary, back up
// the database, bring Xray to the version the release pins, swap the binary and
// restart. Nothing changes on the server until every check has passed.
func (u *SwiftUpdater) Update(ctx context.Context, version string) (err error) {
	version = selfupdate.Normalize(version)
	if version == "" {
		return &ValidationError{Fields: map[string]string{"version": "invalid"}}
	}
	if !u.Info(ctx).Supported {
		return &UpdateError{Code: "unsupported", Detail: u.Version}
	}
	if !u.running.TryLock() {
		return ErrUpdateBusy
	}
	restarting := false
	defer func() {
		if !restarting {
			u.running.Unlock()
		}
		if err != nil {
			u.progress("error", err.Error())
			u.event("error", "swift.update_failed", map[string]any{"version": version, "error": err.Error()})
		}
	}()
	from := u.current()

	u.progress("downloading", version)
	asset := selfupdate.AssetName(version, u.GOOS, u.GOARCH)
	archive, want, err := u.Source.Archive(ctx, version, asset)
	if err != nil {
		return &UpdateError{Code: "download", Detail: err.Error()}
	}
	u.progress("verifying", "")
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return &UpdateError{Code: "checksum", Detail: fmt.Sprintf("sha256 %s, expected %s", got, want)}
	}
	body, err := selfupdate.ExtractTarGz(archive, "swift")
	if err != nil {
		return &UpdateError{Code: "download", Detail: err.Error()}
	}
	next := u.Binary + ".new"
	if err := os.WriteFile(next, body, 0o755); err != nil {
		return fmt.Errorf("write new swift: %w", err)
	}
	defer os.Remove(next)

	u.progress("testing", "")
	out, err := u.probe(ctx, next)
	m := swiftVersionOut.FindStringSubmatch(strings.TrimSpace(out))
	if err != nil || m == nil || selfupdate.Normalize(m[1]) != version {
		return &UpdateError{Code: "rejected", Detail: fmt.Sprintf("the new binary answered %q (%v)", strings.TrimSpace(out), err)}
	}
	newXray := m[2]

	u.progress("backup", "")
	backup := filepath.Join(u.DataDir, fmt.Sprintf("backup-%s-v%s.db", u.Store.Now().Format("20060102-150405"), from))
	if err := u.Store.Backup(ctx, backup); err != nil {
		return &UpdateError{Code: "backup", Detail: err.Error()}
	}

	// a version the administrator installed on purpose stays
	if u.Xray != nil && xray.ValidVersion(newXray) {
		if cur := u.Xray.Current(ctx); cur == xray.Version && cur != newXray {
			u.progress("xray", newXray)
			if err := u.Xray.Update(ctx, newXray); err != nil {
				return &UpdateError{Code: "xray", Detail: err.Error()}
			}
		}
	}

	old := u.Binary + ".bak"
	_ = os.Remove(old) // the previous backup lives until the next update
	if err := os.Rename(u.Binary, old); err != nil {
		return fmt.Errorf("back up swift: %w", err)
	}
	if err := os.Rename(next, u.Binary); err != nil {
		_ = os.Rename(old, u.Binary)
		return fmt.Errorf("install new swift: %w", err)
	}
	if err := u.Store.SetSetting(ctx, swiftFromKey, from); err != nil {
		return err
	}
	u.progress("restarting", version)
	restarting = true
	if u.Restart != nil {
		// let the HTTP reply and the SSE event reach the browser first
		time.AfterFunc(time.Second, u.Restart)
	}
	return nil
}
