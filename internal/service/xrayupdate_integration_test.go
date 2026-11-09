//go:build integration

package service

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestRealXrayUpdateRollsBack installs a binary that passes `xray run -test`
// but does not run; the updater must put the real xray back by itself.
func TestRealXrayUpdateRollsBack(t *testing.T) {
	orig := xray.TestBinary()
	if orig == "" {
		t.Fatal("xray binary not found: run `make xray`")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(lib, xray.BinaryName(runtime.GOOS))
	copyFile(t, orig, bin)
	for _, f := range []string{"geoip.dat", "geosite.dat"} {
		copyFile(t, filepath.Join(filepath.Dir(orig), f), filepath.Join(lib, f))
	}

	broken := filepath.Join(dir, "broken"+filepath.Ext(bin))
	build := exec.Command("go", "build", "-o", broken, "./testdata/brokenxray")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build broken xray: %v\n%s", err, out)
	}
	brokenBody, _ := os.ReadFile(broken)
	archive, sum := zipWith(t, xray.BinaryName(runtime.GOOS), brokenBody)

	st, err := store.Open(ctx, filepath.Join(dir, "swift.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	apiPort := freePort(t)
	_ = st.SetSetting(ctx, "xray_api_port", strconv.Itoa(apiPort))
	priv, _, _ := xray.RealityKeyPair()
	if _, err := st.CreateProfile(ctx, xray.Profile{Name: "Vision", Kind: xray.KindVision, Port: freePort(t), Enabled: true, Params: xray.Params{
		PrivateKey: priv, ShortIDs: xray.ShortIDs(1), Target: "www.example.com:443", ServerNames: []string{"www.example.com"},
	}}); err != nil {
		t.Fatal(err)
	}
	client, err := xray.Dial("127.0.0.1:" + strconv.Itoa(apiPort))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	cfgPath := filepath.Join(dir, "xray", "config.json")
	sup := &xray.Supervisor{Runner: xray.ExecRunner{}, Bin: bin, ConfigPath: cfgPath, Log: discard()}
	x := &Xray{Store: st, Runner: xray.ExecRunner{}, Sup: sup, API: client, Bin: bin, ConfigPath: cfgPath, Log: discard()}
	if err := x.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	go sup.Run(ctx)
	wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
	defer wcancel()
	if err := x.WaitAPI(wctx); err != nil {
		t.Fatal(err)
	}

	u := &XrayUpdater{
		Store: st, Xray: x, Asset: "test.zip", Binary: xray.BinaryName(runtime.GOOS),
		Source: &fakeReleases{latest: "99.0.0", archive: archive, sum: sum},
		Probe:  xray.BinaryVersion,
	}
	before := u.Current(ctx)
	if before != xray.Version {
		t.Fatalf("installed version %q, want %s", before, xray.Version)
	}
	started := time.Now()
	err = u.Update(ctx, "99.0.0")
	var ue *UpdateError
	if !errors.As(err, &ue) || ue.Code != "rolled_back" {
		t.Fatalf("want automatic rollback, got %v", err)
	}
	t.Logf("broken binary detected and rolled back in %v", time.Since(started))
	got, _ := os.ReadFile(bin)
	want, _ := os.ReadFile(orig)
	if !bytes.Equal(got, want) {
		t.Fatal("the real xray binary was not restored")
	}
	wctx2, wcancel2 := context.WithTimeout(ctx, 10*time.Second)
	defer wcancel2()
	if err := x.WaitAPI(wctx2); err != nil {
		t.Fatalf("xray API is not back after rollback: %v", err)
	}
	if sup.Status().Status != xray.StatusRunning || u.Current(ctx) != before {
		t.Fatalf("after rollback: %+v, version %s", sup.Status(), u.Current(ctx))
	}
}
