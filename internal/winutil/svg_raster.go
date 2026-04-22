package winutil

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"time"

	"github.com/nfnt/resize"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"

	_ "image/gif"
	_ "image/jpeg"
	_ "golang.org/x/image/webp"
)

// Default platform logo colors (C# AppSettings TryGetStyle parity stub).
const (
	DefaultPlatformLogoBackground = "#23272A"
	DefaultPlatformLogoForeground = "#FFFFFF"
)

// ApplyPlatformSVGTheming mirrors IconFactory.CreateIcon SVG tweaks when id="FG" is present.
func ApplyPlatformSVGTheming(svg []byte, bgHex, fgHex string) []byte {
	s := string(svg)
	if !strings.Contains(s, `id="FG"`) {
		return svg
	}
	bgHex = strings.TrimSpace(bgHex)
	if bgHex == "" {
		bgHex = DefaultPlatformLogoBackground
	}
	fgHex = strings.TrimSpace(fgHex)
	if fgHex == "" {
		fgHex = DefaultPlatformLogoForeground
	}
	const pathFG = `<path id="FG"`
	if strings.Contains(s, pathFG) {
		rectAndPath := fmt.Sprintf(
			`<rect fill="%s" width="500" height="500"></rect><path id="FG" fill="%s"`,
			bgHex, fgHex,
		)
		s = strings.Replace(s, pathFG, rectAndPath, 1)
	}
	const glass = `<path d="M500,0L0,0L0,500L500,0Z" fill="#FFFFFF" fill-opacity="0.02"/>`
	s = strings.Replace(s, "</svg>", glass+"</svg>", 1)
	return []byte(s)
}

// RasterizeSVGToNRGBA renders SVG to an NRGBA of size×size (oksvg; may fall back to Wails canvas).
func RasterizeSVGToNRGBA(svg []byte, size int) (*image.NRGBA, error) {
	if size <= 0 {
		return nil, fmt.Errorf("invalid size %d", size)
	}
	img, err := rasterizeSVGOK(svg, size)
	if err == nil {
		return img, nil
	}
	pngBytes, werr := RequestSVGRenderViaWails(string(svg), size, 5*time.Second)
	if werr != nil {
		return nil, fmt.Errorf("oksvg: %v; wails: %w", err, werr)
	}
	dec, _, derr := image.Decode(bytes.NewReader(pngBytes))
	if derr != nil {
		return nil, fmt.Errorf("decode wails png: %w", derr)
	}
	return imageToNRGBA(resize.Resize(uint(size), uint(size), dec, resize.Lanczos3)), nil
}

func rasterizeSVGOK(svg []byte, size int) (*image.NRGBA, error) {
	icon, err := oksvg.ReadIconStream(bytes.NewReader(svg), oksvg.IgnoreErrorMode)
	if err != nil {
		return nil, err
	}
	if icon.ViewBox.W <= 0 || icon.ViewBox.H <= 0 {
		icon.ViewBox.W = float64(size)
		icon.ViewBox.H = float64(size)
	}
	icon.SetTarget(0, 0, float64(size), float64(size))

	rgba := image.NewNRGBA(image.Rect(0, 0, size, size))
	scanner := rasterx.NewScannerGV(size, size, rgba, rgba.Bounds())
	dasher := rasterx.NewDasher(size, size, scanner)
	icon.Draw(dasher, 1)
	return rgba, nil
}

// SolidNRGBA fills an NRGBA with a flat color (used when no platform art).
func SolidNRGBA(size int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// EncodePNG encodes img as PNG bytes.
func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
