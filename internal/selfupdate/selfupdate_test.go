package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{"v0.3.0": "0.3.0", "1.2.3": "1.2.3", "dev": "", "v0.3.0-2-gabc": "", "": ""} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestChecksumsAndArchive(t *testing.T) {
	sums := []byte("aa11  swift_0.3.0_linux_arm64.tar.gz\nBB22  swift_0.3.0_linux_amd64.tar.gz\n")
	if s, err := ParseChecksums(sums, AssetName("0.3.0", "linux", "amd64")); err != nil || s != "bb22" {
		t.Fatalf("checksum %q %v", s, err)
	}
	if _, err := ParseChecksums(sums, "other"); err == nil {
		t.Fatal("missing file accepted")
	}

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, body := range map[string]string{"README.md": "readme", "swift": "binary"} {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(body))
	}
	_ = tw.Close()
	_ = zw.Close()
	if b, err := ExtractTarGz(buf.Bytes(), "swift"); err != nil || string(b) != "binary" {
		t.Fatalf("extract %q %v", b, err)
	}
	if _, err := ExtractTarGz(buf.Bytes(), "xray"); err == nil {
		t.Fatal("missing file accepted")
	}
}
