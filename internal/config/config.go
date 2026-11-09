// Package config loads swift settings from a TOML file and SWIFT_* environment variables.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const DefaultPath = "/etc/swift/swift.toml"

// Config holds process-level settings. Product settings (ports, path, domain)
// live in the database; values here only override them when set.
type Config struct {
	DataDir   string // directory for swift.db, xray config and certificates
	LibDir    string // directory with the xray binary and geo files
	XrayBin   string
	Listen    string // host to bind, empty means all interfaces
	PanelPort int
	PanelPath string
	SubPort   int
	Dev       bool
	// InsecureHTTP serves the panel and subscriptions without TLS. Local development only.
	InsecureHTTP bool
	// ACMEStaging uses the Let's Encrypt staging CA (for testing issuance).
	ACMEStaging bool
	ACMEEmail   string
}

func Default() Config {
	return Config{
		DataDir: "/var/lib/swift",
		LibDir:  "/usr/local/lib/swift",
	}
}

func (c Config) DBPath() string { return filepath.Join(c.DataDir, "swift.db") }

func (c Config) XrayPath() string {
	if c.XrayBin != "" {
		return c.XrayBin
	}
	return filepath.Join(c.LibDir, "xray")
}

// Load reads path (a missing file is not an error) and applies environment overrides.
func Load(path string, env func(string) string) (Config, error) {
	c := Default()
	if path != "" {
		f, err := os.Open(path)
		switch {
		case err == nil:
			defer f.Close()
			kv, err := parse(f)
			if err != nil {
				return c, fmt.Errorf("config %s: %w", path, err)
			}
			if err := c.apply(kv); err != nil {
				return c, fmt.Errorf("config %s: %w", path, err)
			}
		case !errors.Is(err, os.ErrNotExist):
			return c, fmt.Errorf("open config: %w", err)
		}
	}
	kv := map[string]string{}
	for _, k := range keys {
		if v := env("SWIFT_" + strings.ToUpper(k)); v != "" {
			kv[k] = v
		}
	}
	if err := c.apply(kv); err != nil {
		return c, fmt.Errorf("environment: %w", err)
	}
	return c, nil
}

var keys = []string{"data_dir", "lib_dir", "xray_bin", "listen", "panel_port", "panel_path", "sub_port", "dev", "insecure_http", "acme_staging", "acme_email"}

func (c *Config) apply(kv map[string]string) error {
	for k, v := range kv {
		var err error
		switch k {
		case "data_dir":
			c.DataDir = v
		case "lib_dir":
			c.LibDir = v
		case "xray_bin":
			c.XrayBin = v
		case "listen":
			c.Listen = v
		case "panel_port":
			c.PanelPort, err = port(v)
		case "panel_path":
			c.PanelPath = strings.Trim(v, "/")
		case "sub_port":
			c.SubPort, err = port(v)
		case "dev":
			c.Dev, err = strconv.ParseBool(v)
		case "insecure_http":
			c.InsecureHTTP, err = strconv.ParseBool(v)
		case "acme_staging":
			c.ACMEStaging, err = strconv.ParseBool(v)
		case "acme_email":
			c.ACMEEmail = v
		default:
			return fmt.Errorf("unknown key %q", k)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
	}
	return nil
}

func port(v string) (int, error) {
	p, err := strconv.Atoi(v)
	if err != nil || p < 1 || p > 65535 {
		return 0, fmt.Errorf("invalid port %q", v)
	}
	return p, nil
}

// parse understands the flat subset of TOML swift needs: key = "string" | int | bool, # comments.
func parse(r io.Reader) (map[string]string, error) {
	kv := map[string]string{}
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key = value", n)
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if strings.HasPrefix(v, `"`) {
			end := strings.Index(v[1:], `"`)
			if end < 0 {
				return nil, fmt.Errorf("line %d: unterminated string", n)
			}
			rest := strings.TrimSpace(v[end+2:])
			if rest != "" && !strings.HasPrefix(rest, "#") {
				return nil, fmt.Errorf("line %d: unexpected %q", n, rest)
			}
			v = v[1 : end+1]
		} else if i := strings.Index(v, "#"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		kv[k] = v
	}
	return kv, sc.Err()
}
