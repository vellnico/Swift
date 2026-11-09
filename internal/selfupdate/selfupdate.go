// Package selfupdate finds and downloads Swift releases from GitHub.
package selfupdate

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var versionRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// Normalize strips the leading "v" of a tag; it returns "" for anything that
// is not a release version (for example "dev" or a git describe string).
func Normalize(v string) string {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if !versionRe.MatchString(v) {
		return ""
	}
	return v
}

// AssetName is the release archive GoReleaser publishes for a platform.
func AssetName(version, goos, goarch string) string {
	return fmt.Sprintf("swift_%s_%s_%s.tar.gz", version, goos, goarch)
}

// ParseChecksums returns the SHA256 of name from a checksums.txt.
func ParseChecksums(b []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("%s is not in checksums.txt", name)
}

// ExtractTarGz returns one regular file of a .tar.gz archive.
func ExtractTarGz(archive []byte, name string) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s not found in archive", name)
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if h.Typeflag == tar.TypeReg && strings.TrimPrefix(h.Name, "./") == name {
			return io.ReadAll(io.LimitReader(tr, 256<<20))
		}
	}
}

// Source finds and downloads releases; replaced in tests.
type Source interface {
	// Latest returns the newest stable version, without "v".
	Latest(ctx context.Context) (string, error)
	// Archive downloads the archive of version and its published SHA256 (hex).
	Archive(ctx context.Context, version, asset string) (archive []byte, sha256Hex string, err error)
}

// GitHub reads releases of the Swift repository.
type GitHub struct {
	Repo     string // default vellnico/Swift
	Client   *http.Client
	API      string // default https://api.github.com
	Download string // default https://github.com
}

func (g GitHub) repo() string {
	if g.Repo != "" {
		return g.Repo
	}
	return "vellnico/Swift"
}

func (g GitHub) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "swift-panel")
	c := g.Client
	if c == nil {
		c = &http.Client{Timeout: 5 * time.Minute}
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

func (g GitHub) Latest(ctx context.Context) (string, error) {
	api := g.API
	if api == "" {
		api = "https://api.github.com"
	}
	body, err := g.get(ctx, api+"/repos/"+g.repo()+"/releases/latest", 1<<20)
	if err != nil {
		return "", err
	}
	var rel struct {
		Tag string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", fmt.Errorf("decode release: %w", err)
	}
	v := Normalize(rel.Tag)
	if v == "" {
		return "", fmt.Errorf("unexpected release tag %q", rel.Tag)
	}
	return v, nil
}

func (g GitHub) Archive(ctx context.Context, version, asset string) ([]byte, string, error) {
	if Normalize(version) == "" {
		return nil, "", fmt.Errorf("invalid version %q", version)
	}
	base := g.Download
	if base == "" {
		base = "https://github.com"
	}
	dir := base + "/" + g.repo() + "/releases/download/v" + version + "/"
	archive, err := g.get(ctx, dir+asset, 256<<20)
	if err != nil {
		return nil, "", err
	}
	sums, err := g.get(ctx, dir+"checksums.txt", 64<<10)
	if err != nil {
		return nil, "", err
	}
	sum, err := ParseChecksums(sums, asset)
	if err != nil {
		return nil, "", err
	}
	return archive, sum, nil
}
