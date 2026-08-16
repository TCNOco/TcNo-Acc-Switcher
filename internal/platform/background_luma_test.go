package platform

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func fillRect(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fillRect(img, img.Bounds(), c)
	return img
}

func closeTo(got, want, tol float64) bool { return math.Abs(got-want) <= tol }

func TestSampleImageLumaSolidColours(t *testing.T) {
	cases := []struct {
		name string
		c    color.RGBA
		want float64
	}{
		{"black", color.RGBA{0, 0, 0, 255}, 0},
		{"white", color.RGBA{255, 255, 255, 255}, 1},
		{"mid grey", color.RGBA{128, 128, 128, 255}, 0.2159},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			luma, err := sampleImageLuma(solid(120, 80, tc.c))
			if err != nil {
				t.Fatalf("sampleImageLuma: %v", err)
			}
			if !luma.Measured {
				t.Fatal("Measured = false, want true")
			}
			if !closeTo(luma.Mean, tc.want, 0.005) {
				t.Fatalf("Mean = %.4f, want ~%.4f", luma.Mean, tc.want)
			}
			// A flat colour has nothing to spread between, so both percentiles
			// land on the mean. This is the case that lets the UI flip text
			// colour with confidence.
			if !closeTo(luma.High-luma.Low, 0, 0.005) {
				t.Fatalf("spread = %.4f, want ~0 for a solid colour", luma.High-luma.Low)
			}
		})
	}
}

// The case that makes a naive average dangerous: half the picture is black and
// half is white, so the mean says "mid grey" while neither black nor white text
// would actually be readable across it. The percentile spread is what exposes it.
func TestSampleImageLumaSplitImageReportsWideSpread(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 120, 80))
	fillRect(img, image.Rect(0, 0, 120, 40), color.RGBA{0, 0, 0, 255})
	fillRect(img, image.Rect(0, 40, 120, 80), color.RGBA{255, 255, 255, 255})

	luma, err := sampleImageLuma(img)
	if err != nil {
		t.Fatalf("sampleImageLuma: %v", err)
	}
	if !closeTo(luma.Mean, 0.5, 0.02) {
		t.Fatalf("Mean = %.4f, want ~0.5", luma.Mean)
	}
	if !closeTo(luma.Low, 0, 0.01) || !closeTo(luma.High, 1, 0.01) {
		t.Fatalf("Low/High = %.4f/%.4f, want ~0/~1", luma.Low, luma.High)
	}
	if luma.High-luma.Low < 0.9 {
		t.Fatalf("spread = %.4f, want a wide spread for a split image", luma.High-luma.Low)
	}
}

func TestSampleImageLumaIgnoresTransparentPixels(t *testing.T) {
	// Left half transparent, right half white. Only the white half is really
	// there, so the result should read as white rather than as half-dark.
	img := image.NewRGBA(image.Rect(0, 0, 120, 80))
	fillRect(img, image.Rect(60, 0, 120, 80), color.RGBA{255, 255, 255, 255})

	luma, err := sampleImageLuma(img)
	if err != nil {
		t.Fatalf("sampleImageLuma: %v", err)
	}
	if !closeTo(luma.Mean, 1, 0.01) {
		t.Fatalf("Mean = %.4f, want ~1 (transparent half ignored)", luma.Mean)
	}
}

func TestSampleImageLumaRejectsUnusableImages(t *testing.T) {
	if _, err := sampleImageLuma(image.NewRGBA(image.Rect(0, 0, 0, 0))); err == nil {
		t.Fatal("expected an error for a zero-sized image")
	}
	if _, err := sampleImageLuma(image.NewRGBA(image.Rect(0, 0, 40, 40))); err == nil {
		t.Fatal("expected an error for a fully transparent image")
	}
}

func TestPercentile(t *testing.T) {
	sorted := []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1}
	if got := percentile(sorted, 0); got != 0 {
		t.Fatalf("p0 = %v, want 0", got)
	}
	if got := percentile(sorted, 1); got != 1 {
		t.Fatalf("p100 = %v, want 1", got)
	}
	if got := percentile(nil, 0.5); got != 0 {
		t.Fatalf("percentile of empty = %v, want 0", got)
	}
}
