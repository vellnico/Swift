// Command fetchxray downloads the pinned Xray-core release for the host
// platform into .cache/xray and verifies its SHA256 digest.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/vellnico/swift/internal/xray"
)

func main() {
	dir := flag.String("dir", ".cache/xray", "destination directory")
	goos := flag.String("os", runtime.GOOS, "target OS")
	goarch := flag.String("arch", runtime.GOARCH, "target architecture")
	flag.Parse()
	if err := fetch(*dir, *goos, *goarch); err != nil {
		fmt.Fprintln(os.Stderr, "fetchxray:", err)
		os.Exit(1)
	}
}

func fetch(dir, goos, goarch string) error {
	name, err := xray.AssetName(goos, goarch)
	if err != nil {
		return err
	}
	bin := "xray"
	if goos == "windows" {
		bin = "xray.exe"
	}
	stamp := filepath.Join(dir, "VERSION")
	if b, err := os.ReadFile(stamp); err == nil && strings.TrimSpace(string(b)) == xray.Version {
		if _, err := os.Stat(filepath.Join(dir, bin)); err == nil {
			fmt.Println("xray", xray.Version, "already in", dir)
			return nil
		}
	}
	base := "https://github.com/XTLS/Xray-core/releases/download/v" + xray.Version + "/"
	archive, err := download(base + name)
	if err != nil {
		return err
	}
	dgst, err := download(base + name + ".dgst")
	if err != nil {
		return err
	}
	want, err := xray.ParseDgst(dgst)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("%s: sha256 mismatch: got %s want %s", name, got, want)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	for _, f := range zr.File {
		switch f.Name {
		case bin, "geoip.dat", "geosite.dat":
		default:
			continue
		}
		if err := extract(f, filepath.Join(dir, f.Name)); err != nil {
			return err
		}
	}
	if err := os.WriteFile(stamp, []byte(xray.Version+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Println("xray", xray.Version, "installed into", dir)
	return nil
}

func download(url string) ([]byte, error) {
	c := &http.Client{Timeout: 5 * time.Minute}
	resp, err := c.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func extract(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
