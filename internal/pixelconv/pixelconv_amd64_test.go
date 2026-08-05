//go:build amd64

package pixelconv

import (
	"bytes"
	"testing"
	"unsafe"
)

// alignedBuffer returns a slice of n bytes whose first byte sits on a 32-byte
// boundary, which the non-temporal kernel requires.
func alignedBuffer(t testing.TB, n int) []byte {
	t.Helper()
	raw := make([]byte, n+vectorAlign)
	offset := int(uintptr(unsafe.Pointer(&raw[0])) % vectorAlign)
	if offset != 0 {
		offset = vectorAlign - offset
	}
	return raw[offset : offset+n : offset+n]
}

func TestAssemblyKernelsMatchScalar(t *testing.T) {
	kernels := []struct {
		name      string
		width     int
		asm       func(dst, src []byte)
		want      func(dst, src []byte)
		supported bool
		align     bool
	}{
		{name: "opaque/AVX2", width: 32, asm: swizzleOpaqueAVX2, want: swizzleOpaqueScalar, supported: hasAVX2},
		{name: "opaque/AVX2NT", width: 32, asm: swizzleOpaqueAVX2NT, want: swizzleOpaqueScalar, supported: hasAVX2, align: true},
		{name: "opaque/SSSE3", width: 16, asm: swizzleOpaqueSSSE3, want: swizzleOpaqueScalar, supported: hasSSSE3},
		{name: "keepAlpha/AVX2", width: 32, asm: swizzleKeepAlphaAVX2, want: swizzleKeepAlphaScalar, supported: hasAVX2},
		{name: "keepAlpha/SSSE3", width: 16, asm: swizzleKeepAlphaSSSE3, want: swizzleKeepAlphaScalar, supported: hasSSSE3},
	}

	for _, kernel := range kernels {
		t.Run(kernel.name, func(t *testing.T) {
			if !kernel.supported {
				t.Skip("instruction set unavailable on this CPU")
			}
			for _, n := range kernelLengths() {
				src := randomBGRA(t, n)
				var got []byte
				if kernel.align {
					got = alignedBuffer(t, n)
				} else {
					got = make([]byte, n)
				}
				want := make([]byte, n)

				// The kernels deliberately stop at the last whole vector; the
				// Go dispatcher owns the tail, so only compare that prefix.
				covered := n &^ (kernel.width - 1)
				kernel.asm(got, src)
				kernel.want(want[:covered], src[:covered])
				if !bytes.Equal(got[:covered], want[:covered]) {
					t.Fatalf("n=%d: vector prefix disagrees with scalar", n)
				}
				for i := covered; i < n; i++ {
					if got[i] != 0 {
						t.Fatalf("n=%d: kernel wrote past its last whole vector at byte %d", n, i)
					}
				}
			}
		})
	}
}

// The dispatcher's job is picking a kernel and stitching head/body/tail back
// together, including the non-temporal split that only triggers past 8 MB.
func TestDispatchMatchesScalarAcrossNTThreshold(t *testing.T) {
	if !hasAVX2 {
		t.Skip("no AVX2 on this CPU")
	}
	sizes := []int{
		ntStoreThreshold - 4,
		ntStoreThreshold,
		ntStoreThreshold + 4,
		ntStoreThreshold + 132,
	}
	for _, n := range sizes {
		src := randomBGRA(t, n)
		want := make([]byte, n)
		swizzleOpaqueScalar(want, src)

		// Offset the destination by one pixel so the dispatcher has to peel an
		// unaligned head before the non-temporal body.
		raw := make([]byte, n+pixelSize)
		got := raw[pixelSize:]
		swizzleOpaqueAVX2Dispatch(got, src)
		if !bytes.Equal(got, want) {
			t.Fatalf("n=%d: dispatch mismatch", n)
		}
	}
}

// TestDispatchFallbackPaths runs the public API with the CPU feature flags
// forced off, which is the only way to exercise the SSSE3-only and
// SSE2-baseline paths on a machine that has AVX2. One binary has to be correct
// on all three, so all three are checked here rather than left to whichever
// CPU happens to run the suite.
func TestDispatchFallbackPaths(t *testing.T) {
	originalAVX2, originalSSSE3 := hasAVX2, hasSSSE3
	t.Cleanup(func() { hasAVX2, hasSSSE3 = originalAVX2, originalSSSE3 })

	paths := []struct {
		name        string
		avx2, ssse3 bool
	}{
		{name: "AVX2", avx2: originalAVX2, ssse3: originalSSSE3},
		{name: "SSSE3", avx2: false, ssse3: originalSSSE3},
		{name: "SWAR", avx2: false, ssse3: false},
	}
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			if path.name == "AVX2" && !originalAVX2 {
				t.Skip("no AVX2 on this CPU")
			}
			if path.name == "SSSE3" && !originalSSSE3 {
				t.Skip("no SSSE3 on this CPU")
			}
			hasAVX2, hasSSSE3 = path.avx2, path.ssse3

			// Span the NT threshold so the largest path is covered too.
			for _, n := range []int{4, 60, 132, 4096, ntStoreThreshold + 132} {
				src := randomBGRA(t, n)
				want := make([]byte, n)
				got := make([]byte, n)

				swizzleOpaqueScalar(want, src)
				BGRAToNRGBAOpaque(got, src)
				if !bytes.Equal(got, want) {
					t.Fatalf("n=%d: opaque mismatch on the %s path", n, path.name)
				}

				swizzleKeepAlphaScalar(want, src)
				BGRAToNRGBA(got, src)
				if !bytes.Equal(got, want) {
					t.Fatalf("n=%d: keep-alpha mismatch on the %s path", n, path.name)
				}
			}
		})
	}
}

func TestAlignHeadRejectsUnalignablePixelStart(t *testing.T) {
	raw := make([]byte, 64)
	// A one-byte offset can never reach a 32-byte boundary in whole pixels, so
	// alignHead must hand the entire buffer to the scalar path rather than let
	// a non-temporal store run misaligned.
	if head := alignHead(raw[1:]); head != len(raw)-1 {
		t.Fatalf("alignHead = %d, want %d", head, len(raw)-1)
	}
	if head := alignHead(nil); head != 0 {
		t.Fatalf("alignHead(nil) = %d, want 0", head)
	}
}
