package winutil

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/nfnt/resize"

	_ "golang.org/x/image/webp"
)

const combinedIconCanvas = 500

// BuildCombinedAccountIcon writes a multi-size .ico (16,32,48,256) with platform background and avatar at bottom-right (half size), C# IconFactory parity.
//
// Optional cachedPlatformICO: when non-empty and the file exists, it must be the same
// multi-size .ico written by BuildPlatformIcon for this platform (e.g. …/IconCache/steam_platform.ico).
// Decoding that file yields the exact same background tile as the Desktop platform shortcut, then the
// account avatar is composited on top. If the file is missing, art is resolved from the embedded dist
// (SVG/PNG) like BuildPlatformIcon.
func BuildCombinedAccountIcon(platformKey, avatarPath, outICO string, cachedPlatformICO ...string) error {
	platformKey = strings.TrimSpace(platformKey)
	avatarPath = strings.TrimSpace(avatarPath)
	if platformKey == "" || avatarPath == "" {
		return fmt.Errorf("missing platform or avatar path")
	}

	var bg *image.NRGBA
	if len(cachedPlatformICO) > 0 {
		if p := strings.TrimSpace(cachedPlatformICO[0]); p != "" {
			if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
				if im, err := decodeBestFromICO(b); err == nil {
					bg = imageToNRGBA(resize.Resize(combinedIconCanvas, combinedIconCanvas, im, resize.Lanczos3))
				}
			}
		}
	}

	if bg == nil {
		svgBytes, pngBytes, artErr := FindPlatformArt(platformKey)
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
	}

	return writeCombinedICOFromBG(bg, avatarPath, outICO)
}

// BuildCombinedGameIcon writes a multi-size .ico with the game/app tile as background and the
// account avatar at bottom-right (half size), same layout as BuildCombinedAccountIcon.
func BuildCombinedGameIcon(gameIconPath, avatarPath, outICO string) error {
	gameIconPath = strings.TrimSpace(gameIconPath)
	avatarPath = strings.TrimSpace(avatarPath)
	if gameIconPath == "" || avatarPath == "" {
		return fmt.Errorf("missing game icon or avatar path")
	}
	b, err := os.ReadFile(gameIconPath)
	if err != nil {
		return fmt.Errorf("read game icon: %w", err)
	}
	im, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("decode game icon: %w", err)
	}
	bg := imageToNRGBA(resize.Resize(combinedIconCanvas, combinedIconCanvas, im, resize.Lanczos3))
	return writeCombinedICOFromBG(bg, avatarPath, outICO)
}

func writeCombinedICOFromBG(bg *image.NRGBA, avatarPath, outICO string) error {
	avatarPath = strings.TrimSpace(avatarPath)
	if bg == nil || avatarPath == "" {
		return fmt.Errorf("missing background or avatar path")
	}
	fg, err := decodeAvatarImage(avatarPath)
	if err != nil {
		return fmt.Errorf("avatar: %w", err)
	}

	sizes := []int{16, 32, 48, 256}
	pngs := make([][]byte, 0, len(sizes))
	for _, sz := range sizes {
		layer := composePlatformAndAvatar(bg, fg, sz)
		pngBytes, err := EncodePNG(layer)
		if err != nil {
			return err
		}
		pngs = append(pngs, pngBytes)
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

func decodeAvatarImage(path string) (image.Image, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	im, _, err := image.Decode(bytes.NewReader(b))
	return im, err
}

func imageToNRGBA(img image.Image) *image.NRGBA {
	if nr, ok := img.(*image.NRGBA); ok {
		return nr
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}

func composePlatformAndAvatar(bg *image.NRGBA, fg image.Image, size int) *image.NRGBA {
	rb := imageToNRGBA(resize.Resize(uint(size), uint(size), bg, resize.Lanczos3))
	half := size / 2
	rf := imageToNRGBA(resize.Resize(uint(half), uint(half), fg, resize.Lanczos3))
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.Draw(out, out.Bounds(), rb, image.Point{}, draw.Src)
	pt := image.Pt(size-half, size-half)
	draw.Draw(out, image.Rect(pt.X, pt.Y, size, size), rf, image.Point{}, draw.Over)
	return out
}
