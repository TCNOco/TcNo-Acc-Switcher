package winutil

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/nfnt/resize"

	_ "golang.org/x/image/webp"
)

// BuildPlatformIcon writes a multi-size .ico (16, 32, 48, 256) from platform tile art only (no account overlay).
func BuildPlatformIcon(platformKey, outICO string) error {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return fmt.Errorf("missing platform key")
	}
	outICO = strings.TrimSpace(outICO)
	if outICO == "" {
		return fmt.Errorf("missing output path")
	}

	svgBytes, pngBytes, artErr := FindPlatformArt(platformKey)
	var bg *image.NRGBA
	switch {
	case len(svgBytes) > 0:
		themed := ApplyPlatformSVGTheming(svgBytes, DefaultPlatformLogoBackground, DefaultPlatformLogoForeground)
		var err error
		bg, err = RasterizeSVGToNRGBA(themed, combinedIconCanvas)
		if err != nil {
			return fmt.Errorf("rasterize platform svg: %w", err)
		}
	case len(pngBytes) > 0:
		dec, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			return fmt.Errorf("decode platform png: %w", err)
		}
		bg = imageToNRGBA(resize.Resize(combinedIconCanvas, combinedIconCanvas, dec, resize.Lanczos3))
	default:
		if artErr != nil && artErr != ErrNoPlatformArt && artErr != ErrNoEmbeddedFrontend {
			return fmt.Errorf("no platform art: %w", artErr)
		}
		bg = SolidNRGBA(combinedIconCanvas, color.NRGBA{R: 0x23, G: 0x27, B: 0x2A, A: 0xff})
	}

	sizes := []int{16, 32, 48, 256}
	pngs := make([][]byte, 0, len(sizes))
	for _, sz := range sizes {
		layer := imageToNRGBA(resize.Resize(uint(sz), uint(sz), bg, resize.Lanczos3))
		b, err := EncodePNG(layer)
		if err != nil {
			return err
		}
		pngs = append(pngs, b)
	}

	if err := os.MkdirAll(filepath.Dir(outICO), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outICO)
	if err != nil {
		return err
	}
	defer f.Close()
	return SavePNGsAsICO(f, pngs)
}
