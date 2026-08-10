// Package qr renders a tag's found-URL as a QR code in two forms: a resolution-
// independent SVG (ideal for print at any size) and a rasterized PNG at a caller-
// chosen pixel size (for thermal printers / quick previews).
package qr

import (
	"bytes"
	"fmt"

	qrcode "github.com/skip2/go-qrcode"
)

// PNG renders the content as a PNG of sizePx by sizePx pixels. The size is
// clamped to a sane range so a hostile query param can't request a gigapixel
// image.
func PNG(content string, sizePx int) ([]byte, error) {
	if sizePx < 128 {
		sizePx = 128
	}
	if sizePx > 4096 {
		sizePx = 4096
	}
	q, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	q.DisableBorder = false
	return q.PNG(sizePx)
}

// SVG renders the content as a scalable vector QR code. A single <rect> per dark
// module keeps the file compact and crisp at any print size. quietZone is the
// margin in modules (4 is the spec-recommended minimum).
func SVG(content string) ([]byte, error) {
	q, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	bitmap := q.Bitmap() // [][]bool including the quiet-zone border
	n := len(bitmap)
	if n == 0 {
		return nil, fmt.Errorf("empty qr bitmap")
	}

	var b bytes.Buffer
	// viewBox is in module units; the QR scales to whatever width/height the
	// embedding context or print pipeline sets.
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" `+
		`viewBox="0 0 %d %d" shape-rendering="crispEdges" `+
		`width="%d" height="%d" role="img" aria-label="QR code">`, n, n, n*16, n*16)
	// White background so it prints correctly on any surface.
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#ffffff"/>`, n, n)
	b.WriteString(`<path fill="#000000" d="`)
	for y := 0; y < n; y++ {
		row := bitmap[y]
		for x := 0; x < len(row); x++ {
			if row[x] {
				// 1x1 module square at (x,y)
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x, y)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.Bytes(), nil
}
