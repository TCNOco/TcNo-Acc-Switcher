package platform

import (
	"errors"
	"image"
	"io"
	"math"
	"os"
	"sort"

	// Decoders for every extension bgAllowedExts permits. webp has no stdlib
	// decoder, so it comes from x/image.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// BackgroundLuma summarises how bright a background image is, so the UI can pick
// text colours that survive being laid over it.
//
// Low and High are the 10th and 90th percentiles rather than the extremes: a
// single white pixel in a night photo should not make the picture read as
// "bright". The gap between them is what tells the UI whether one text colour
// can work across the whole image or whether it needs a scrim instead.
type BackgroundLuma struct {
	// Measured distinguishes "never sampled" from "sampled, and it came out
	// black". Without it a pure black wallpaper is indistinguishable from an
	// unmeasured one, since both leave Mean at zero.
	Measured bool    `json:"measured,omitempty"`
	Mean     float64 `json:"mean,omitempty"`
	Low      float64 `json:"low,omitempty"`
	High     float64 `json:"high,omitempty"`
}

// lumaGrid is the sampling resolution per axis. 48x48 is far more than a mean and
// a pair of percentiles need, and it avoids allocating a resized copy of what may
// be a 4K photo.
const lumaGrid = 48

// srgbChannel converts one 0..1 sRGB channel to linear light, per WCAG 2.x.
// Mirrors relativeLuminance in frontend/src/lib/theme/color.ts — the two must
// agree or the backend and the canvas fallback would disagree about the same image.
func srgbChannel(v float64) float64 {
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func relativeLuminance(r, g, b float64) float64 {
	return 0.2126*srgbChannel(r) + 0.7152*srgbChannel(g) + 0.0722*srgbChannel(b)
}

// percentile returns the p-quantile (0..1) of an already-sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)-1))
	if idx < 0 {
		idx = 0
	} else if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// sampleImageLuma walks a fixed grid over img and reports its brightness spread.
func sampleImageLuma(img image.Image) (BackgroundLuma, error) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return BackgroundLuma{}, errors.New("background image has no pixels")
	}

	values := make([]float64, 0, lumaGrid*lumaGrid)
	var sum float64
	for gy := 0; gy < lumaGrid; gy++ {
		for gx := 0; gx < lumaGrid; gx++ {
			// Sample the centre of each cell so a grid line never lands on the
			// very edge of the picture.
			x := bounds.Min.X + (gx*w)/lumaGrid + w/(2*lumaGrid)
			y := bounds.Min.Y + (gy*h)/lumaGrid + h/(2*lumaGrid)
			if x >= bounds.Max.X {
				x = bounds.Max.X - 1
			}
			if y >= bounds.Max.Y {
				y = bounds.Max.Y - 1
			}

			r16, g16, b16, a16 := img.At(x, y).RGBA()
			if a16 == 0 {
				// Fully transparent: the theme shows through here, not the image.
				continue
			}
			// RGBA() is alpha-premultiplied; undo that to get the true colour.
			alpha := float64(a16)
			lum := relativeLuminance(
				float64(r16)/alpha,
				float64(g16)/alpha,
				float64(b16)/alpha,
			)
			values = append(values, lum)
			sum += lum
		}
	}

	if len(values) == 0 {
		return BackgroundLuma{}, errors.New("background image is fully transparent")
	}

	sort.Float64s(values)
	return BackgroundLuma{
		Measured: true,
		Mean:     sum / float64(len(values)),
		Low:      percentile(values, 0.10),
		High:     percentile(values, 0.90),
	}, nil
}

// computeBackgroundLuma decodes the image at path and measures its brightness.
// Callers treat an error as "unmeasured" and let the frontend fall back to
// sampling the loaded image on a canvas.
func computeBackgroundLuma(path string) (BackgroundLuma, error) {
	f, err := os.Open(path)
	if err != nil {
		return BackgroundLuma{}, err
	}
	defer f.Close()

	img, _, err := image.Decode(io.LimitReader(f, bgMaxBytes))
	if err != nil {
		return BackgroundLuma{}, err
	}
	return sampleImageLuma(img)
}
