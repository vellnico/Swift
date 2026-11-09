package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileAndEnv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "swift.toml")
	src := `# comment
data_dir = "/srv/swift" # trailing
panel_port = 24000
dev = true
`
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"SWIFT_PANEL_PATH": "/abc/", "SWIFT_PANEL_PORT": "25000"}
	c, err := Load(p, func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if c.DataDir != "/srv/swift" || c.PanelPort != 25000 || c.PanelPath != "abc" || !c.Dev {
		t.Fatalf("unexpected config: %+v", c)
	}
	if c.LibDir != "/usr/local/lib/swift" {
		t.Fatalf("default lib dir lost: %q", c.LibDir)
	}
}

func TestLoadMissingFile(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "none.toml"), func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if c != Default() {
		t.Fatalf("expected defaults, got %+v", c)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []string{"panel_port = 70000\n", "bogus = 1\n", "data_dir = \"x\n", "no equals\n"}
	for _, src := range cases {
		p := filepath.Join(t.TempDir(), "c.toml")
		if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(p, func(string) string { return "" }); err == nil {
			t.Errorf("expected error for %q", src)
		}
	}
}
