package pixelconv

import (
	"fmt"
	"testing"
)

// benchSizes are the pixel counts the QR capture path actually produces: a
// Steam-sized window, then the common monitor resolutions a full-screen
// capture yields, up to the 24-megapixel ceiling qrimage enforces. 1440p and
// ultrawide straddle the non-temporal store threshold, so they are the sizes
// that show whether it is placed correctly.
var benchSizes = []struct {
	name   string
	pixels int
}{
	{name: "640x480", pixels: 640 * 480},
	{name: "1920x1080", pixels: 1920 * 1080},
	{name: "2560x1440", pixels: 2560 * 1440},
	{name: "3440x1440", pixels: 3440 * 1440},
	{name: "3840x2160", pixels: 3840 * 2160},
	{name: "24MP", pixels: 24_000_000},
}

func benchmarkKernel(b *testing.B, pixels int, fn func(dst, src []byte)) {
	n := pixels * pixelSize
	src := randomBGRA(b, n)
	dst := make([]byte, n)
	b.SetBytes(int64(n))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fn(dst, src)
	}
}

func BenchmarkBGRAToNRGBAOpaque(b *testing.B) {
	kernels := []struct {
		name string
		fn   func(dst, src []byte)
	}{
		{name: "Scalar", fn: swizzleOpaqueScalar},
		{name: "SWAR", fn: swizzleOpaqueSWAR},
		{name: "Dispatch", fn: BGRAToNRGBAOpaque},
	}
	for _, size := range benchSizes {
		for _, kernel := range kernels {
			b.Run(fmt.Sprintf("%s/%s", size.name, kernel.name), func(b *testing.B) {
				benchmarkKernel(b, size.pixels, kernel.fn)
			})
		}
	}
}

func BenchmarkBGRAToNRGBAKeepAlpha(b *testing.B) {
	kernels := []struct {
		name string
		fn   func(dst, src []byte)
	}{
		{name: "Scalar", fn: swizzleKeepAlphaScalar},
		{name: "SWAR", fn: swizzleKeepAlphaSWAR},
		{name: "Dispatch", fn: BGRAToNRGBA},
	}
	for _, size := range benchSizes {
		for _, kernel := range kernels {
			b.Run(fmt.Sprintf("%s/%s", size.name, kernel.name), func(b *testing.B) {
				benchmarkKernel(b, size.pixels, kernel.fn)
			})
		}
	}
}
