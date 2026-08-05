package winutil

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"testing"
)

// decodeXOR32Scalar is the byte-at-a-time form decodeXOR32 replaced. It is the
// benchmark baseline and the oracle the vector path is checked against.
func decodeXOR32Scalar(img *image.NRGBA, data []byte, base, stride, w, h int) {
	for vy := 0; vy < h; vy++ {
		bmpRow := h - 1 - vy
		row := base + bmpRow*stride
		for vx := 0; vx < w; vx++ {
			o := row + vx*4
			img.SetNRGBA(vx, vy, color.NRGBA{R: data[o+2], G: data[o+1], B: data[o], A: data[o+3]})
		}
	}
}

func xorPlane(t testing.TB, size int) []byte {
	t.Helper()
	data := make([]byte, size*4*size)
	rng := rand.New(rand.NewSource(int64(size)))
	if _, err := rng.Read(data); err != nil {
		t.Fatalf("seed pixels: %v", err)
	}
	return data
}

// icoSizes are the icon dimensions BuildCombinedAccountIcon actually writes.
var icoSizes = []int{16, 32, 48, 256}

func BenchmarkDecodeXOR32(b *testing.B) {
	for _, size := range icoSizes {
		data := xorPlane(b, size)
		stride := size * 4
		img := image.NewNRGBA(image.Rect(0, 0, size, size))

		for _, variant := range []struct {
			name string
			fn   func(*image.NRGBA, []byte, int, int, int, int)
		}{
			{name: "Scalar", fn: decodeXOR32Scalar},
			{name: "Vector", fn: decodeXOR32},
		} {
			b.Run(fmt.Sprintf("%dx%d/%s", size, size, variant.name), func(b *testing.B) {
				b.SetBytes(int64(len(data)))
				for i := 0; i < b.N; i++ {
					variant.fn(img, data, 0, stride, size, size)
				}
			})
		}
	}
}

// TestDecodeXOR32MatchesScalar pins the vector path to the original per-pixel
// decode, including the bottom-up row flip and odd widths whose rows end
// mid-vector.
func TestDecodeXOR32MatchesScalar(t *testing.T) {
	for _, size := range []int{1, 3, 5, 16, 17, 47, 48, 256} {
		data := xorPlane(t, size)
		stride := size * 4
		want := image.NewNRGBA(image.Rect(0, 0, size, size))
		got := image.NewNRGBA(image.Rect(0, 0, size, size))
		decodeXOR32Scalar(want, data, 0, stride, size, size)
		decodeXOR32(got, data, 0, stride, size, size)
		if !bytes.Equal(got.Pix, want.Pix) {
			t.Fatalf("size=%d: vector decode disagrees with scalar", size)
		}
	}
}
