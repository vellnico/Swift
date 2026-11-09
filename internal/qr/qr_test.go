package qr

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
)

func TestPNGAndSVG(t *testing.T) {
	data := "http://203.0.113.7:24680/s/abcdefghijklmnopqrstuvwxyz012345"
	b, err := PNG(data, 256)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil || img.Bounds().Dx() != 256 {
		t.Fatalf("png: %v", err)
	}
	s, err := SVG(data)
	if err != nil {
		t.Fatal(err)
	}
	str := string(s)
	if !strings.HasPrefix(str, "<svg") || !strings.Contains(str, `fill="#000"`) || !strings.Contains(str, `fill="#fff"`) || strings.Count(str, "M") < 50 {
		t.Fatalf("svg looks wrong: %.120s", str)
	}
}
