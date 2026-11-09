package xray

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AssetName is the release archive of Xray-core for a platform.
func AssetName(goos, goarch string) (string, error) {
	arch := map[string]string{"amd64": "64", "arm64": "arm64-v8a", "386": "32"}[goarch]
	if arch == "" {
		return "", fmt.Errorf("unsupported arch %s", goarch)
	}
	switch goos {
	case "linux", "windows", "freebsd", "openbsd":
		return fmt.Sprintf("Xray-%s-%s.zip", goos, arch), nil
	case "darwin":
		return "Xray-macos-" + arch + ".zip", nil
	}
	return "", fmt.Errorf("unsupported os %s", goos)
}

// BinaryName is the executable inside the release archive.
func BinaryName(goos string) string {
	if goos == "windows" {
		return "xray.exe"
	}
	return "xray"
}

// ParseDgst reads the "SHA2-256= <hex>" line of an Xray .dgst file.
func ParseDgst(b []byte) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if ok && strings.TrimSpace(k) == "SHA2-256" {
			return strings.ToLower(strings.TrimSpace(v)), nil
		}
	}
	return "", errors.New("no SHA2-256 in digest file")
}

// ExtractFile returns one file of a zip archive.
func ExtractFile(archive []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(io.LimitReader(rc, 512<<20))
	}
	return nil, fmt.Errorf("%s not found in archive", name)
}

var versionRe = regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,3}$`)

// ValidVersion accepts release versions like 26.10.31 (no "v").
func ValidVersion(v string) bool { return versionRe.MatchString(v) }

// CompareVersions compares dotted numeric versions: -1, 0 or 1.
func CompareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
	}
	return 0
}

// MajorChanged reports a change of the first version component, which may
// bring config schema changes the generator does not know about.
func MajorChanged(from, to string) bool {
	f, _, _ := strings.Cut(from, ".")
	t, _, _ := strings.Cut(to, ".")
	return f != t
}

// ReleaseSource finds and downloads Xray-core releases; replaced in tests.
type ReleaseSource interface {
	// Latest returns the newest stable version, without "v".
	Latest(ctx context.Context) (string, error)
	// Archive downloads the archive of version and its expected SHA256 (hex).
	Archive(ctx context.Context, version, asset string) (archive []byte, sha256Hex string, err error)
}

// GitHubReleases reads XTLS/Xray-core releases from GitHub.
type GitHubReleases struct {
	Client   *http.Client
	API      string // default https://api.github.com/repos/XTLS/Xray-core
	Download string // default https://github.com/XTLS/Xray-core/releases/download
}

func (g GitHubReleases) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (g GitHubReleases) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "swift-panel")
	resp, err := g.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

func (g GitHubReleases) Latest(ctx context.Context) (string, error) {
	api := g.API
	if api == "" {
		api = "https://api.github.com/repos/XTLS/Xray-core"
	}
	body, err := g.get(ctx, api+"/releases/latest", 1<<20)
	if err != nil {
		return "", err
	}
	var rel struct {
		Tag string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", fmt.Errorf("decode release: %w", err)
	}
	v := strings.TrimPrefix(rel.Tag, "v")
	if !ValidVersion(v) {
		return "", fmt.Errorf("unexpected release tag %q", rel.Tag)
	}
	return v, nil
}

func (g GitHubReleases) Archive(ctx context.Context, version, asset string) ([]byte, string, error) {
	if !ValidVersion(version) {
		return nil, "", fmt.Errorf("invalid version %q", version)
	}
	base := g.Download
	if base == "" {
		base = "https://github.com/XTLS/Xray-core/releases/download"
	}
	url := base + "/v" + version + "/" + asset
	archive, err := g.get(ctx, url, 256<<20)
	if err != nil {
		return nil, "", err
	}
	dgst, err := g.get(ctx, url+".dgst", 64<<10)
	if err != nil {
		return nil, "", err
	}
	sum, err := ParseDgst(dgst)
	if err != nil {
		return nil, "", err
	}
	return archive, sum, nil
}

var binaryVersionRe = regexp.MustCompile(`Xray ([0-9]+(?:\.[0-9]+){1,3})`)

// BinaryVersion runs `xray version` and returns e.g. "26.10.31".
func BinaryVersion(ctx context.Context, bin string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "version").Output()
	if err != nil {
		return "", fmt.Errorf("xray version: %w", err)
	}
	m := binaryVersionRe.FindSubmatch(out)
	if m == nil {
		return "", fmt.Errorf("xray version: unexpected output %q", lastLines(string(out), 1))
	}
	return string(m[1]), nil
}
