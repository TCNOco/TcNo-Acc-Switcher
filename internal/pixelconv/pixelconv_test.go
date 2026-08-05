package pixelconv

import (
	"bytes"
	"math/rand"
	"testing"
)

func randomBGRA(t testing.TB, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	rng := rand.New(rand.NewSource(int64(n) + 1))
	if _, err := rng.Read(buf); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	return buf
}

// Lengths sweep every unroll boundary and every sub-vector tail: below one
// SSE vector, between the SSE and AVX widths, inside the 128-byte block, and
// past it with each possible remainder.
func kernelLengths() []int {
	lengths := make([]int, 0, 320)
	for n := 0; n <= 288; n += 4 {
		lengths = append(lengths, n)
	}
	return append(lengths, 512, 1024, 4092, 65536, 1<<20)
}

func TestSWARMatchesScalar(t *testing.T) {
	for _, n := range kernelLengths() {
		src := randomBGRA(t, n)
		want := make([]byte, n)
		got := make([]byte, n)

		swizzleOpaqueScalar(want, src)
		swizzleOpaqueSWAR(got, src)
		if !bytes.Equal(got, want) {
			t.Fatalf("opaque SWAR mismatch at n=%d", n)
		}

		swizzleKeepAlphaScalar(want, src)
		swizzleKeepAlphaSWAR(got, src)
		if !bytes.Equal(got, want) {
			t.Fatalf("keep-alpha SWAR mismatch at n=%d", n)
		}
	}
}

func TestPublicAPIMatchesScalar(t *testing.T) {
	for _, n := range kernelLengths() {
		src := randomBGRA(t, n)
		want := make([]byte, n)
		got := make([]byte, n)

		swizzleOpaqueScalar(want, src)
		BGRAToNRGBAOpaque(got, src)
		if !bytes.Equal(got, want) {
			t.Fatalf("BGRAToNRGBAOpaque mismatch at n=%d", n)
		}

		swizzleKeepAlphaScalar(want, src)
		BGRAToNRGBA(got, src)
		if !bytes.Equal(got, want) {
			t.Fatalf("BGRAToNRGBA mismatch at n=%d", n)
		}
	}
}

// The public API is the seam that decides how many bytes to touch, so a short
// destination must stop early rather than panic or spill into whole pixels the
// caller did not provide room for.
func TestConversionTruncatesToWholePixels(t *testing.T) {
	cases := []struct {
		dst, src, want int
	}{
		{dst: 16, src: 16, want: 16},
		{dst: 15, src: 16, want: 12},
		{dst: 16, src: 15, want: 12},
		{dst: 3, src: 64, want: 0},
		{dst: 0, src: 64, want: 0},
		{dst: 64, src: 0, want: 0},
		{dst: 37, src: 40, want: 36},
		{dst: 131, src: 4096, want: 128},
	}
	for _, tc := range cases {
		src := randomBGRA(t, tc.src)
		dst := bytes.Repeat([]byte{0xAB}, tc.dst)
		BGRAToNRGBAOpaque(dst, src)

		reference := make([]byte, tc.dst)
		copy(reference, bytes.Repeat([]byte{0xAB}, tc.dst))
		swizzleOpaqueScalar(reference[:tc.want], src[:tc.want])
		if !bytes.Equal(dst, reference) {
			t.Fatalf("dst=%d src=%d: converted region disagrees with %d-byte expectation", tc.dst, tc.src, tc.want)
		}
		for i := tc.want; i < tc.dst; i++ {
			if dst[i] != 0xAB {
				t.Fatalf("dst=%d src=%d: byte %d past the whole-pixel boundary was overwritten", tc.dst, tc.src, i)
			}
		}
	}
}

// In-place conversion is what the ICO decoder and any future single-buffer
// caller would do; PSHUFB reads its whole vector before storing, so identical
// slices are safe and this pins that down.
func TestInPlaceConversion(t *testing.T) {
	for _, n := range []int{4, 64, 128, 260, 65536} {
		src := randomBGRA(t, n)
		want := make([]byte, n)
		swizzleOpaqueScalar(want, src)

		buf := append([]byte(nil), src...)
		BGRAToNRGBAOpaque(buf, buf)
		if !bytes.Equal(buf, want) {
			t.Fatalf("in-place opaque conversion mismatch at n=%d", n)
		}
	}
}
