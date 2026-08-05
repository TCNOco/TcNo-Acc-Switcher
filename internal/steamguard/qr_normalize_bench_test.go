package steamguard

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"TcNo-Acc-Switcher/internal/steamguard/qrimage"
)

// normalizeBGRAFrameScalar is the byte-at-a-time conversion normalizeBGRAFrame
// replaced. It is the benchmark baseline and the oracle for the vector path.
func normalizeBGRAFrameScalar(width, height, stride int, source []byte) (*qrimage.Frame, bool) {
	if width <= 0 || height <= 0 || stride != width*4 ||
		int64(width)*int64(height) > qrimage.MaxCandidatePixels ||
		len(source) != stride*height {
		return nil, false
	}
	destination := &qrimage.Frame{
		Width:  width,
		Height: height,
		Stride: stride,
		Pixels: make([]byte, len(source)),
	}
	for offset := 0; offset < len(source); offset += 4 {
		destination.Pixels[offset] = source[offset+2]
		destination.Pixels[offset+1] = source[offset+1]
		destination.Pixels[offset+2] = source[offset]
		destination.Pixels[offset+3] = 0xff
	}
	return destination, true
}

// captureSizes are the per-monitor capture dimensions the QR scan path feeds
// through normalization, up to qrimage's 24-megapixel ceiling.
var captureSizes = []struct {
	name          string
	width, height int
}{
	{name: "640x480", width: 640, height: 480},
	{name: "1920x1080", width: 1920, height: 1080},
	{name: "2560x1440", width: 2560, height: 1440},
	{name: "3840x2160", width: 3840, height: 2160},
	{name: "5120x4320", width: 5120, height: 4320},
}

func captureBuffer(t testing.TB, width, height int) []byte {
	t.Helper()
	buf := make([]byte, width*height*4)
	rng := rand.New(rand.NewSource(int64(width*height) + 1))
	if _, err := rng.Read(buf); err != nil {
		t.Fatalf("seed capture: %v", err)
	}
	return buf
}

func BenchmarkNormalizeBGRAFrame(b *testing.B) {
	for _, size := range captureSizes {
		source := captureBuffer(b, size.width, size.height)
		stride := size.width * 4

		for _, variant := range []struct {
			name string
			fn   func(int, int, int, []byte) (*qrimage.Frame, bool)
		}{
			{name: "Scalar", fn: normalizeBGRAFrameScalar},
			{name: "Vector", fn: normalizeBGRAFrame},
		} {
			b.Run(fmt.Sprintf("%s/%s", size.name, variant.name), func(b *testing.B) {
				b.SetBytes(int64(len(source)))
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					frame, ok := variant.fn(size.width, size.height, stride, source)
					if !ok {
						b.Fatal("normalization rejected a valid frame")
					}
					frame.Wipe()
				}
			})
		}
	}
}

func TestNormalizeBGRAFrameMatchesScalar(t *testing.T) {
	for _, size := range captureSizes[:3] {
		source := captureBuffer(t, size.width, size.height)
		stride := size.width * 4

		want, okWant := normalizeBGRAFrameScalar(size.width, size.height, stride, source)
		got, okGot := normalizeBGRAFrame(size.width, size.height, stride, source)
		if !okWant || !okGot {
			t.Fatalf("%s: normalization rejected a valid frame", size.name)
		}
		if !bytes.Equal(got.Pixels, want.Pixels) {
			t.Fatalf("%s: vector normalization disagrees with scalar", size.name)
		}
	}
}

// The guard clauses decide whether untrusted capture geometry is converted at
// all, so they are checked independently of the conversion itself.
func TestNormalizeBGRAFrameRejectsBadGeometry(t *testing.T) {
	cases := []struct {
		name                  string
		width, height, stride int
		sourceLen             int
	}{
		{name: "zero width", width: 0, height: 8, stride: 0, sourceLen: 0},
		{name: "zero height", width: 8, height: 0, stride: 32, sourceLen: 0},
		{name: "negative width", width: -4, height: 8, stride: -16, sourceLen: 0},
		{name: "stride not four bytes per pixel", width: 8, height: 4, stride: 24, sourceLen: 96},
		{name: "source shorter than geometry", width: 8, height: 4, stride: 32, sourceLen: 64},
		{name: "source longer than geometry", width: 8, height: 4, stride: 32, sourceLen: 256},
		{name: "over the pixel ceiling", width: 8192, height: 8192, stride: 32768, sourceLen: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if frame, ok := normalizeBGRAFrame(tc.width, tc.height, tc.stride, make([]byte, tc.sourceLen)); ok || frame != nil {
				t.Fatal("invalid geometry was accepted")
			}
		})
	}
}
