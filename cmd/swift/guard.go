package main

import (
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vellnico/swift/internal/config"
	"github.com/vellnico/swift/internal/service"
)

// updateGuard is started by the panel from the previous binary (swift.bak)
// right before it restarts into a new version; it rolls the update back if
// the new version does not confirm itself (SPEC 11.5).
func updateGuard(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("update-guard", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath, "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath, os.Getenv)
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if p, err := filepath.EvalSymlinks(self); err == nil {
		self = p
	}
	g := &service.UpdateGuard{
		DBPath:      cfg.DBPath(),
		Binary:      strings.TrimSuffix(self, ".bak"),
		XrayBin:     cfg.XrayPath(),
		Unit:        "swift",
		Systemd:     service.ExecSystemd{},
		Log:         stdout,
		Timeout:     3 * time.Minute,
		Interval:    2 * time.Second,
		MaxRestarts: 3,
	}
	return g.Run(context.Background())
}
