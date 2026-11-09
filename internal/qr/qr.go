// Package qr renders QR codes as PNG and SVG: pure black on pure white so
// phone cameras read them reliably (SPEC 10.2).
package qr

import (
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

func PNG(data string, size int) ([]byte, error) {
	b, err := qrcode.Encode(data, qrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("qr png: %w", err)
	}
	return b, nil
}

// SVG draws the code as one path of unit squares, scaled by the viewer.
func SVG(data string) ([]byte, error) {
	q, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("qr svg: %w", err)
	}
	bm := q.Bitmap() // includes the quiet zone
	n := len(bm)
	var path strings.Builder
	for y, row := range bm {
		for x := 0; x < len(row); {
			if !row[x] {
				x++
				continue
			}
			start := x
			for x < len(row) && row[x] {
				x++
			}
			fmt.Fprintf(&path, "M%d %dh%dv1h-%dz", start, y, x-start, x-start)
		}
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges"><rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="%s"/></svg>`,
		n, n, n, n, path.String())
	return []byte(svg), nil
}
