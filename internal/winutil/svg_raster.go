package winutil

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nfnt/resize"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"

	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/webp"
)

// DefaultPlatformLogoForeground matches in-app accent (#80ffea) so rasterised icons align with the UI.
const (
	DefaultPlatformLogoBackground = "#23272A"
	DefaultPlatformLogoForeground = "#80ffea"
)

var (
	// id="FG" may appear after other attributes (e.g. d= first).
	pathFGOpenRE = regexp.MustCompile(`(?i)<path(\s+[^>]*\bid\s*=\s*["']FG["'][^>]*)(/?>)`)
	svgAttrFill  = regexp.MustCompile(`(?i)\bfill\s*=`)
	// width/height using percentage or other non-absolute units break oksvg and
	// <img src=data:...> intrinsic sizing; coerce using viewBox.
	svgWidthPercentRE  = regexp.MustCompile(`(?i)\s+width\s*=\s*["'][^"']*%[^"']*["']`)
	svgHeightPercentRE = regexp.MustCompile(`(?i)\s+height\s*=\s*["'][^"']*%[^"']*["']`)
	svgHasWidthRE      = regexp.MustCompile(`(?i)\s+width\s*=`)
	svgHasHeightRE     = regexp.MustCompile(`(?i)\s+height\s*=`)
	viewBoxRE          = regexp.MustCompile(`(?i)viewBox\s*=\s*["']([^"']+)["']`)
)

func parseViewBoxSize(s string) (w, h int, ok bool) {
	m := viewBoxRE.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0, 0, false
	}
	fields := strings.Fields(strings.TrimSpace(m[1]))
	if len(fields) < 4 {
		return 0, 0, false
	}
	fw, err1 := strconv.ParseFloat(fields[len(fields)-2], 64)
	fh, err2 := strconv.ParseFloat(fields[len(fields)-1], 64)
	if err1 != nil || err2 != nil || fw <= 0 || fh <= 0 {
		return 0, 0, false
	}
	return int(fw + 0.5), int(fh + 0.5), true
}

// coerceSVGRootPixelDimensions forces numeric width/height on the root <svg>
// so rasterisers get a definite viewport (percent sizes often rasterise empty).
func coerceSVGRootPixelDimensions(s string, fallback int) string {
	iw, ih, ok := parseViewBoxSize(s)
	if !ok {
		iw, ih = fallback, fallback
	}
	start := strings.Index(s, "<svg")
	if start < 0 {
		return s
	}
	rel := strings.Index(s[start:], ">")
	if rel < 0 {
		return s
	}
	tagStart, tagEnd := start, start+rel
	tag := s[tagStart:tagEnd]

	tag = svgWidthPercentRE.ReplaceAllString(tag, fmt.Sprintf(` width="%d"`, iw))
	tag = svgHeightPercentRE.ReplaceAllString(tag, fmt.Sprintf(` height="%d"`, ih))
	if !svgHasWidthRE.MatchString(tag) {
		tag += fmt.Sprintf(` width="%d"`, iw)
	}
	if !svgHasHeightRE.MatchString(tag) {
		tag += fmt.Sprintf(` height="%d"`, ih)
	}
	return s[:tagStart] + tag + s[tagEnd:]
}

// ApplyPlatformSVGTheming: when a <path id="FG"> exists we wrap it with a solid
// background <rect> and force a fill on the FG path. For SVGs that don't tag a
// foreground path (and don't set fill themselves), we inject a default fill on
// the <svg> root so all descendant paths inherit the accent colour instead of
// SVG's implicit black.
func ApplyPlatformSVGTheming(svg []byte, bgHex, fgHex string) []byte {
	s := string(svg)
	bgHex = strings.TrimSpace(bgHex)
	if bgHex == "" {
		bgHex = DefaultPlatformLogoBackground
	}
	fgHex = strings.TrimSpace(fgHex)
	if fgHex == "" {
		fgHex = DefaultPlatformLogoForeground
	}

	s = coerceSVGRootPixelDimensions(s, combinedIconCanvas)

	iw, ih, vbOK := parseViewBoxSize(s)
	if !vbOK {
		iw, ih = combinedIconCanvas, combinedIconCanvas
	}

	if m := pathFGOpenRE.FindStringSubmatch(s); len(m) >= 3 {
		full := m[0]
		attrs := m[1]
		suffix := m[2]
		if !svgAttrFill.MatchString(attrs) {
			attrs = ` fill="` + fgHex + `"` + attrs
		}
		rect := fmt.Sprintf(
			`<rect fill="%s" width="%d" height="%d" x="0" y="0"></rect><path`,
			bgHex, iw, ih,
		)
		s = strings.Replace(s, full, rect+attrs+suffix, 1)
		const glass = `<path d="M500,0L0,0L0,500L500,0Z" fill="#FFFFFF" fill-opacity="0.02"/>`
		s = strings.Replace(s, "</svg>", glass+"</svg>", 1)
		return []byte(s)
	}

	return []byte(ensureSVGRootFill(s, fgHex))
}

// ensureSVGRootFill adds a fill="..." attribute to the root <svg> element
// when one isn't already present, so paths without an explicit fill inherit
// fgHex instead of rendering black.
func ensureSVGRootFill(s, fgHex string) string {
	start := strings.Index(s, "<svg")
	if start < 0 {
		return s
	}
	rel := strings.Index(s[start:], ">")
	if rel < 0 {
		return s
	}
	tagEnd := start + rel
	tag := s[start:tagEnd]
	if strings.Contains(tag, " fill=") || strings.Contains(tag, "\tfill=") {
		return s
	}
	insertAt := tagEnd
	if tagEnd > 0 && s[tagEnd-1] == '/' {
		insertAt = tagEnd - 1
	}
	return s[:insertAt] + fmt.Sprintf(` fill="%s"`, fgHex) + s[insertAt:]
}

// ForceWailsFallbackSVG forces canvas rasterization instead of oksvg (debug).
const ForceWailsFallbackSVG = false

// RasterizeSVGToNRGBA renders SVG to an NRGBA of size×size (oksvg; may fall back to Wails canvas).
func RasterizeSVGToNRGBA(svg []byte, size int) (*image.NRGBA, error) {
	if size <= 0 {
		return nil, fmt.Errorf("invalid size %d", size)
	}
	var okErr error
	if ForceWailsFallbackSVG {
		okErr = fmt.Errorf("skipped (ForceWailsFallbackSVG=true)")
	} else {
		img, err := rasterizeSVGOK(svg, size)
		if err == nil {
			return img, nil
		}
		okErr = err
	}
	pngBytes, werr := RequestSVGRenderViaWails(string(svg), size, 5*time.Second)
	if werr != nil {
		return nil, fmt.Errorf("oksvg: %v; wails: %w", okErr, werr)
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
	bg := color.NRGBA{R: 0x23, G: 0x27, B: 0x2A, A: 0xff}
	draw.Draw(rgba, rgba.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)

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
