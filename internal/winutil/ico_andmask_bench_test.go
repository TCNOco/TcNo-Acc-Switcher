package winutil

import (
	"bytes"
	"fmt"
	"image"
	"math/rand"
	"testing"
)

// andMaskShapes covers the two ends of what real icons carry. Modern 32bpp
// icons keep transparency in the alpha channel and ship an all-zero AND mask,
// so "opaque" is the common case; "random" is the branch-predictor worst case.
var andMaskShapes = []struct {
	name string
	fill func([]byte, int)
}{
	{name: "opaque", fill: func(b []byte, _ int) { clear(b) }},
	// A palette icon's mask is long runs of clear and set bits, not noise, so
	// this is the realistic shape when a mask is actually used.
	{name: "cutout", fill: func(b []byte, _ int) {
		clear(b)
		for i := range b[:len(b)/2] {
			b[i] = 0xFF
		}
	}},
	{name: "random", fill: func(b []byte, seed int) {
		rng := rand.New(rand.NewSource(int64(seed)))
		_, _ = rng.Read(b)
	}},
}

// applyANDMaskScalar is the per-pixel form applyANDMask replaced, kept as the
// benchmark baseline and as an independent oracle. It reads the plane bottom-up
// like the production version; this oracle only pins the per-pixel arithmetic,
// not the row order, which TestICOPaletteANDMaskRowOrientation covers.
func applyANDMaskScalar(img *image.NRGBA, data []byte, base, stride, w, h int) {
	for vy := 0; vy < h; vy++ {
		row := base + (h-1-vy)*stride
		for vx := 0; vx < w; vx++ {
			bit := uint(vx % 8)
			b := data[row+vx/8]
			if (b>>(7-bit))&1 == 0 {
				continue
			}
			c := img.NRGBAAt(vx, vy)
			c.A = 0
			img.SetNRGBA(vx, vy, c)
		}
	}
}

func BenchmarkApplyANDMask(b *testing.B) {
	for _, size := range icoSizes {
		stride := bmpStride(size, 1)
		for _, shape := range andMaskShapes {
			mask := make([]byte, stride*size)
			shape.fill(mask, size)

			for _, variant := range []struct {
				name string
				fn   func(*image.NRGBA, []byte, int, int, int, int)
			}{
				{name: "Scalar", fn: applyANDMaskScalar},
				{name: "Fast", fn: applyANDMask},
			} {
				b.Run(fmt.Sprintf("%dx%d/%s/%s", size, size, shape.name, variant.name), func(b *testing.B) {
					img := image.NewNRGBA(image.Rect(0, 0, size, size))
					for i := 0; i < b.N; i++ {
						variant.fn(img, mask, 0, stride, size, size)
					}
				})
			}
		}
	}
}

func TestApplyANDMaskMatchesScalar(t *testing.T) {
	// Widths that are not multiples of eight leave a partial mask byte whose
	// padding bits must not clear real pixels.
	for _, size := range []int{1, 3, 7, 8, 9, 17, 47, 48, 256} {
		stride := bmpStride(size, 1)
		mask := make([]byte, stride*size)
		rng := rand.New(rand.NewSource(int64(size)))
		if _, err := rng.Read(mask); err != nil {
			t.Fatalf("seed mask: %v", err)
		}
		pixels := xorPlane(t, size)

		want := image.NewNRGBA(image.Rect(0, 0, size, size))
		got := image.NewNRGBA(image.Rect(0, 0, size, size))
		decodeXOR32(want, pixels, 0, size*4, size, size)
		decodeXOR32(got, pixels, 0, size*4, size, size)

		applyANDMaskScalar(want, mask, 0, stride, size, size)
		applyANDMask(got, mask, 0, stride, size, size)
		if !bytes.Equal(got.Pix, want.Pix) {
			t.Fatalf("size=%d: rewritten AND mask disagrees with scalar", size)
		}
	}
}
