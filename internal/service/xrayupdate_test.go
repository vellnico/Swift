package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vellnico/swift/internal/xray"
)

type fakeReleases struct {
	latest  string
	archive []byte
	sum     string
}

func (f *fakeReleases) Latest(context.Context) (string, error) { return f.latest, nil }
func (f *fakeReleases) Archive(context.Context, string, string) ([]byte, string, error) {
	return f.archive, f.sum, nil
}

func zipWith(t *testing.T, name string, body []byte) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write(body)
	_ = zw.Close()
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:])
}

// binAPI answers only while the installed binary is not "broken".
type binAPI struct {
	*api
	bin string
}

func (b binAPI) Ping(ctx context.Context) error {
	body, _ := os.ReadFile(b.bin)
	if string(body) == "broken" {
		return errors.New("unavailable")
	}
	return b.api.Ping(ctx)
}

type rejectingRunner struct {
	*runner
	bad string
}

func (r rejectingRunner) Test(ctx context.Context, bin, path string) error {
	if body, _ := os.ReadFile(bin); string(body) == r.bad {
		return &xray.TestError{Output: "unknown field", Err: errors.New("exit 23")}
	}
	return r.runner.Test(ctx, bin, path)
}

func updaterSetup(t *testing.T, newBinary string) (*XrayUpdater, *fakeReleases, *[]string) {
	t.Helper()
	x, r, a, _ := setup(t)
	x.Bin = filepath.Join(filepath.Dir(x.ConfigPath), "xray")
	if err := os.WriteFile(x.Bin, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	x.Sup.Bin = x.Bin
	x.API = binAPI{a, x.Bin}
	x.Runner = rejectingRunner{r, "rejected"}
	archive, sum := zipWith(t, "xray", []byte(newBinary))
	src := &fakeReleases{latest: "26.11.5", archive: archive, sum: sum}
	var mu sync.Mutex
	var stages []string
	u := &XrayUpdater{
		Store: x.Store, Xray: x, Source: src, Asset: "Xray-linux-64.zip", Binary: "xray",
		Probe:     func(context.Context, string) (string, error) { return "26.10.31", nil },
		UpTimeout: 300 * time.Millisecond,
		Progress: func(s, _ string) {
			mu.Lock()
			stages = append(stages, s)
			mu.Unlock()
		},
	}
	return u, src, &stages
}

func binary(t *testing.T, u *XrayUpdater) string {
	t.Helper()
	b, err := os.ReadFile(u.Xray.Bin)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestXrayUpdateSucceeds(t *testing.T) {
	u, _, stages := updaterSetup(t, "new")
	ctx := context.Background()
	info, err := u.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Major || info.Latest != "26.11.5" || info.Current != "26.10.31" {
		t.Fatalf("check: %+v", info)
	}
	if err := u.Update(ctx, "26.11.5"); err != nil {
		t.Fatal(err)
	}
	if binary(t, u) != "new" {
		t.Fatal("binary was not replaced")
	}
	if b, _ := os.ReadFile(u.Xray.Bin + ".bak"); string(b) != "old" {
		t.Fatal("previous binary is not kept as backup")
	}
	info = u.Info(ctx)
	if info.Current != "26.11.5" || info.Available || !info.Backup {
		t.Fatalf("after update: %+v", info)
	}
	if got := strings.Join(*stages, ","); got != "downloading,verifying,testing,restarting,done" {
		t.Fatalf("stages %s", got)
	}
}

func TestXrayUpdateRollsBackBrokenBinary(t *testing.T) {
	u, _, stages := updaterSetup(t, "broken")
	ctx := context.Background()
	var events []string
	u.Event = func(_, _, key string, _ map[string]any) { events = append(events, key) }
	err := u.Update(ctx, "26.11.5")
	var ue *UpdateError
	if !errors.As(err, &ue) || ue.Code != "rolled_back" {
		t.Fatalf("want rolled_back, got %v", err)
	}
	if binary(t, u) != "old" {
		t.Fatal("old binary was not restored")
	}
	if u.Info(ctx).Current != "26.10.31" {
		t.Fatal("version changed after rollback")
	}
	wctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := u.Xray.WaitAPI(wctx); err != nil {
		t.Fatalf("xray is not back after rollback: %v", err)
	}
	if !strings.HasSuffix(strings.Join(*stages, ","), "restarting,rolled_back") || strings.Join(events, ",") != "xray.update_rolled_back" {
		t.Fatalf("stages %v events %v", *stages, events)
	}
}

func TestXrayUpdateRejectsBadChecksumAndConfig(t *testing.T) {
	u, src, _ := updaterSetup(t, "rejected")
	ctx := context.Background()
	err := u.Update(ctx, "26.11.5")
	var ue *UpdateError
	if !errors.As(err, &ue) || ue.Code != "rejected" {
		t.Fatalf("want rejected, got %v", err)
	}
	src.sum = strings.Repeat("0", 64)
	if err := u.Update(ctx, "26.11.5"); !errors.As(err, &ue) || ue.Code != "checksum" {
		t.Fatalf("want checksum, got %v", err)
	}
	if binary(t, u) != "old" {
		t.Fatal("binary changed by a failed update")
	}
	if _, err := os.Stat(u.Xray.Bin + ".new"); err == nil {
		t.Fatal("temporary binary left behind")
	}
	if err := u.Update(ctx, "../26"); err == nil {
		t.Fatal("invalid version accepted")
	}
}

func TestXrayMajorUpdateIsFlagged(t *testing.T) {
	u, src, _ := updaterSetup(t, "new")
	src.latest = "27.1.1"
	info, err := u.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || !info.Major {
		t.Fatalf("major update not flagged: %+v", info)
	}
}

func TestReleaseHelpers(t *testing.T) {
	if xray.CompareVersions("26.10.31", "26.9.9") != 1 || xray.CompareVersions("26.1", "26.1.0") != 0 || xray.CompareVersions("25.12.8", "26.1.1") != -1 {
		t.Fatal("CompareVersions")
	}
	if sum, err := xray.ParseDgst([]byte("MD5= aa\nSHA2-256= ABCDEF\n")); err != nil || sum != "abcdef" {
		t.Fatalf("ParseDgst %q %v", sum, err)
	}
	if xray.ValidVersion("v26.1.1") || xray.ValidVersion("26") || !xray.ValidVersion("26.10.31") {
		t.Fatal("ValidVersion")
	}
}
