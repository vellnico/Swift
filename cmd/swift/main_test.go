package main

import (
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/vellnico/swift/internal/config"
	"github.com/vellnico/swift/internal/store"
)

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"version"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "swift dev\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"bogus"}, &out, &out); err == nil {
		t.Fatal("expected error")
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
