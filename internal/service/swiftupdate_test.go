package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vellnico/swift/internal/store"
)

type fakeSwiftSource struct {
	latest  string
	archive []byte
	sum     string
}

func (f *fakeSwiftSource) Latest(context.Context) (string, error) { return f.latest, nil }

func (f *fakeSwiftSource) Archive(_ context.Context, version, asset string) ([]byte, string, error) {
	if asset != "swift_"+version+"_linux_amd64.tar.gz" {
		return nil, "", errors.New("unexpected asset " + asset)
	}
	return f.archive, f.sum, nil
}

func tarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = zw.Close()
	return buf.Bytes()
}

func swiftEnv(t *testing.T) (*SwiftUpdater, *fakeSwiftSource, *atomic.Int32, *[]string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(context.Background(), filepath.Join(dir, "swift.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	bin := filepath.Join(dir, "swift")
	if err := os.WriteFile(bin, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	archive := tarGz(t, "swift", []byte("new"))
	sum := sha256.Sum256(archive)
	src := &fakeSwiftSource{latest: "0.4.0", archive: archive, sum: hex.EncodeToString(sum[:])}
	restarts := &atomic.Int32{}
	events := &[]string{}
	u := &SwiftUpdater{
		Store: st, Source: src, Version: "v0.3.0", Binary: bin, DataDir: dir, GOOS: "linux", GOARCH: "amd64",
		Probe:   func(context.Context, string) (string, error) { return "swift v0.4.0 xray 26.6.27\n", nil },
		Restart: func() { restarts.Add(1) },
		Event:   func(_, _, key string, _ map[string]any) { *events = append(*events, key) },
	}
	return u, src, restarts, events
}

func TestSwiftUpdateSwapsBinaryAndRestarts(t *testing.T) {
	ctx := context.Background()
	u, _, restarts, events := swiftEnv(t)
	if info := u.Info(ctx); !info.Supported || info.Available || info.Auto {
		t.Fatalf("before check: %+v", info)
	}
	info, err := u.Check(ctx)
	if err != nil || !info.Available || info.Latest != "0.4.0" || info.Current != "0.3.0" {
		t.Fatalf("check: %+v %v", info, err)
	}
	if err := u.Update(ctx, "0.4.0"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(u.Binary); string(b) != "new" {
		t.Fatalf("binary %q", b)
	}
	if b, _ := os.ReadFile(u.Binary + ".bak"); string(b) != "old" {
		t.Fatalf("backup binary %q", b)
	}
	if m, _ := filepath.Glob(filepath.Join(u.DataDir, "backup-*-v0.3.0.db")); len(m) != 1 {
		t.Fatalf("database backups %v", m)
	}
	deadline := time.Now().Add(3 * time.Second)
	for restarts.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no restart after the update")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !u.Info(ctx).Updating {
		t.Fatal("a second update must wait for the restart")
	}

	// the new process reports the update once
	next := &SwiftUpdater{Store: u.Store, Version: "v0.4.0", GOOS: "linux", Event: u.Event}
	next.ReportStartup(ctx)
	next.ReportStartup(ctx)
	if len(*events) != 1 || (*events)[0] != "swift.updated" {
		t.Fatalf("events %v", *events)
	}
}

func TestSwiftUpdateRefusesBadReleases(t *testing.T) {
	ctx := context.Background()
	u, src, restarts, _ := swiftEnv(t)
	src.sum = "00"
	var ue *UpdateError
	if err := u.Update(ctx, "0.4.0"); !errors.As(err, &ue) || ue.Code != "checksum" {
		t.Fatalf("checksum: %v", err)
	}
	sum := sha256.Sum256(src.archive)
	src.sum = hex.EncodeToString(sum[:])
	u.Probe = func(context.Context, string) (string, error) { return "swift v0.3.9 xray 26.6.27", nil }
	if err := u.Update(ctx, "0.4.0"); !errors.As(err, &ue) || ue.Code != "rejected" {
		t.Fatalf("wrong version inside: %v", err)
	}
	if b, _ := os.ReadFile(u.Binary); string(b) != "old" || restarts.Load() != 0 {
		t.Fatal("a refused update changed the server")
	}
	if _, err := os.Stat(u.Binary + ".new"); !os.IsNotExist(err) {
		t.Fatal("temporary binary left behind")
	}
	dev := &SwiftUpdater{Store: u.Store, Source: src, Version: "dev", GOOS: "linux"}
	if dev.Info(ctx).Supported || !errors.As(dev.Update(ctx, "0.4.0"), &ue) || ue.Code != "unsupported" {
		t.Fatal("development builds must not update themselves")
	}
}

func TestSwiftAutoUpdateOnlyAtNight(t *testing.T) {
	ctx := context.Background()
	u, _, _, _ := swiftEnv(t)
	if err := u.SetAuto(ctx, true); err != nil || !u.Info(ctx).Auto {
		t.Fatalf("auto: %v", err)
	}
	u.nightly(ctx, false) // the check right after start finds the release but waits for the night
	if b, _ := os.ReadFile(u.Binary); string(b) != "old" {
		t.Fatal("updated outside the night")
	}
	u.nightly(ctx, true)
	if b, _ := os.ReadFile(u.Binary); string(b) != "new" {
		t.Fatal("auto-update did not install the release")
	}
}

func TestNextNight(t *testing.T) {
	loc := time.FixedZone("test", 3*3600)
	at := func(h, m int) time.Time { return time.Date(2026, 11, 9, h, m, 0, 0, loc) }
	for _, tc := range []struct {
		now  time.Time
		want time.Time
	}{
		{at(1, 0), at(3, 30)},
		{at(3, 29), at(3, 30)},
		{at(3, 30), at(3, 30).AddDate(0, 0, 1)},
		{at(23, 0), at(3, 30).AddDate(0, 0, 1)},
	} {
		if got := NextNight(tc.now, 30*time.Minute); !got.Equal(tc.want) {
			t.Errorf("NextNight(%v) = %v, want %v", tc.now, got, tc.want)
		}
	}
}
