package xray

import (
	"os"
	"path/filepath"
	"runtime"
)

// TestBinary locates the xray binary used by tests: $SWIFT_XRAY_BIN or
// .cache/xray in the repository (downloaded by `make xray`). Empty if absent.
func TestBinary() string {
	if p := os.Getenv("SWIFT_XRAY_BIN"); p != "" {
		return p
	}
	name := "xray"
	if runtime.GOOS == "windows" {
		name = "xray.exe"
	}
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			p := filepath.Join(dir, ".cache", "xray", name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
