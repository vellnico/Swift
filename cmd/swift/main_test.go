package main

import (
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/vellnico/swift/internal/config"
	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"version"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "swift dev xray ") {
		t.Fatalf("got %q", out.String())
	}
	out.Reset()
	_ = run([]string{"version", "--xray"}, &out, &out)
	if strings.TrimSpace(out.String()) != xray.Version {
		t.Fatalf("--xray: %q", out.String())
	}
}

func TestSetupIsIdempotentAndBackupWorks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SWIFT_DATA_DIR", dir)
	bin := xray.TestBinary()
	if bin == "" {
		t.Fatal("xray binary not found: run `make xray`")
	}
	t.Setenv("SWIFT_XRAY_BIN", bin)
	var out bytes.Buffer
	// no default profiles: they probe Reality targets over the network
	if err := run([]string{"setup", "-config", "", "-address", "203.0.113.7", "-no-profiles", "-no-ip-cert"}, &out, &out); err != nil {
		t.Fatalf("setup: %v\n%s", err, out.String())
	}
	out.Reset()
	if err := run([]string{"setup", "-config", ""}, &out, &out); err != nil || !strings.Contains(out.String(), "already set up") {
		t.Fatalf("second setup: %v %q", err, out.String())
	}
	backup := filepath.Join(dir, "backup.db")
	if err := run([]string{"db", "backup", backup, "-config", ""}, &out, &out); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(context.Background(), backup)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if v, _ := st.Setting(context.Background(), "server_address"); v != "203.0.113.7" {
		t.Fatalf("backup content: %q", v)
	}
	if err := run([]string{"db", "backup", backup, "-config", ""}, &out, &out); err == nil {
		t.Fatal("backup must not overwrite an existing file")
	}
}

func TestUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"bogus"}, &out, &out); err == nil {
		t.Fatal("expected error")
	}
}

func TestAdminResetAndShow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SWIFT_DATA_DIR", dir)
	var out bytes.Buffer
	if err := run([]string{"admin", "show", "-config", ""}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("swift admin reset")) {
		t.Fatalf("show without admin: %q", out.String())
	}
	out.Reset()
	if err := run([]string{"admin", "reset", "-config", ""}, &out, &out); err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`Login:\s+([a-z0-9]{8})\s+Password: (\S{20})`).FindStringSubmatch(out.String())
	if m == nil {
		t.Fatalf("reset output: %q", out.String())
	}
	out.Reset()
	if err := run([]string{"admin", "show", "-config", ""}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Login: "+m[1])) || bytes.Contains(out.Bytes(), []byte(m[2])) {
		t.Fatalf("show must print the login and never the password: %q", out.String())
	}
}

func TestPanelAddressGeneratedOnceAndPersisted(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "swift.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	port, path, err := panelAddress(ctx, config.Default(), st)
	if err != nil {
		t.Fatal(err)
	}
	if port < 20000 || port > 60000 || !regexp.MustCompile(`^[a-z0-9]{16}$`).MatchString(path) {
		t.Fatalf("bad address %d %q", port, path)
	}
	port2, path2, err := panelAddress(ctx, config.Default(), st)
	if err != nil || port2 != port || path2 != path {
		t.Fatalf("not persisted: %d %q %v", port2, path2, err)
	}
	cfg := config.Default()
	cfg.PanelPort, cfg.PanelPath = 1234, "x"
	if p, s, _ := panelAddress(ctx, cfg, st); p != 1234 || s != "x" {
		t.Fatalf("config override ignored: %d %q", p, s)
	}
}

func TestSetupStoresLanguageOnEveryRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SWIFT_DATA_DIR", dir)
	t.Setenv("SWIFT_XRAY_BIN", filepath.Join(dir, "no-xray"))
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(dir, "swift.db"))
	if err != nil {
		t.Fatal(err)
	}
	// a finished setup must still accept a new language
	if err := st.SetSetting(ctx, "setup_done", "1"); err != nil {
		t.Fatal(err)
	}
	st.Close()
	var out bytes.Buffer
	if err := run([]string{"setup", "-config", "", "-lang", "de"}, &out, &out); err == nil {
		t.Fatal("unknown language accepted")
	}
	if err := run([]string{"setup", "-config", "", "-lang", "ru"}, &out, &out); err != nil {
		t.Fatalf("setup: %v %s", err, out.String())
	}
	if _, err := run2(t, "admin", "reset", "-config", ""); err != nil {
		t.Fatal(err)
	}
	show, err := run2(t, "admin", "show", "-config", "")
	if err != nil || !strings.Contains(show, "Language: ru\n") {
		t.Fatalf("admin show: %v %q", err, show)
	}
}

func run2(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(args, &out, &out)
	return out.String(), err
}
