package routing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GeoUpdater refreshes the server's geoip.dat/geosite.dat from runetfreedom
// once a day; files are verified against the published SHA-256 sums.
type GeoUpdater struct {
	Dir    string // next to the xray binary (XRAY_LOCATION_ASSET)
	Client *http.Client
	Base   string // overridable in tests; defaults to the release branch
	Event  func(level, key string, params map[string]any)
}

func (g *GeoUpdater) event(level, key string, p map[string]any) {
	if g.Event != nil {
		g.Event(level, key, p)
	}
}

func (g *GeoUpdater) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c := g.Client
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Minute}
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// Update downloads both files; a file whose checksum does not match is skipped.
func (g *GeoUpdater) Update(ctx context.Context) error {
	base := g.Base
	if base == "" {
		base = releaseBase
	}
	var firstErr error
	for _, name := range []string{"geoip.dat", "geosite.dat"} {
		if err := g.updateOne(ctx, base, name); err != nil {
			g.event("warn", "geo.update_failed", map[string]any{"file": name, "error": err.Error()})
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if firstErr == nil {
		g.event("info", "geo.updated", nil)
	}
	return firstErr
}

func (g *GeoUpdater) updateOne(ctx context.Context, base, name string) error {
	sumFile, err := g.get(ctx, base+name+".sha256sum", 4096)
	if err != nil {
		return err
	}
	want := strings.ToLower(strings.Fields(string(sumFile) + " ")[0])
	if len(want) != 64 {
		return fmt.Errorf("%s: malformed checksum", name)
	}
	body, err := g.get(ctx, base+name, 512<<20)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("%s: sha256 mismatch", name)
	}
	path := filepath.Join(g.Dir, name)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Run updates now and then every interval until ctx ends.
func (g *GeoUpdater) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		_ = g.Update(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
