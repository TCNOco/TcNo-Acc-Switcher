//go:build amd64

package pixelconv

import (
	"unsafe"

	"golang.org/x/sys/cpu"
)

// ntStoreThreshold is the destination size past which the AVX2 kernel switches
// to non-temporal stores. Below it the write-allocate traffic is absorbed by
// cache and normal stores win; above it bypassing the cache removes the
// read-for-ownership fetch of a buffer nothing reads before it is overwritten.
//
// Measured crossover on a 13900K sits between a 1080p frame (8.3 MB, where
// cached stores hit 29 GB/s against the non-temporal path's 19-23 GB/s) and a
// 1440p frame (14.7 MB, where that reverses to 15-17 GB/s against 24-30 GB/s).
// 12 MB sits between them with margin, so no common capture size lands on the
// boundary by a rounding coincidence.
const ntStoreThreshold = 12 << 20

// vectorAlign is the destination alignment VMOVNTDQ requires.
const vectorAlign = 32

//go:noescape
func swizzleOpaqueAVX2(dst, src []byte)

//go:noescape
func swizzleOpaqueAVX2NT(dst, src []byte)

//go:noescape
func swizzleOpaqueSSSE3(dst, src []byte)

//go:noescape
func swizzleKeepAlphaAVX2(dst, src []byte)

//go:noescape
func swizzleKeepAlphaSSSE3(dst, src []byte)

var (
	hasAVX2  = cpu.X86.HasAVX2
	hasSSSE3 = cpu.X86.HasSSSE3
)

func swizzleOpaque(dst, src []byte) {
	switch {
	case hasAVX2:
		swizzleOpaqueAVX2Dispatch(dst, src)
	case hasSSSE3:
		swizzleOpaqueSSSE3(dst, src)
		done := len(dst) &^ 15
		swizzleOpaqueSWAR(dst[done:], src[done:])
	default:
		swizzleOpaqueSWAR(dst, src)
	}
}

func swizzleKeepAlpha(dst, src []byte) {
	switch {
	case hasAVX2:
		swizzleKeepAlphaAVX2(dst, src)
		done := len(dst) &^ 31
		swizzleKeepAlphaSWAR(dst[done:], src[done:])
	case hasSSSE3:
		swizzleKeepAlphaSSSE3(dst, src)
		done := len(dst) &^ 15
		swizzleKeepAlphaSWAR(dst[done:], src[done:])
	default:
		swizzleKeepAlphaSWAR(dst, src)
	}
}

// swizzleOpaqueAVX2Dispatch splits the buffer so the non-temporal kernel only
// ever sees a 32-byte-aligned destination. The unaligned head and the sub-vector
// tail go through the cached kernel and the SWAR fallback respectively.
func swizzleOpaqueAVX2Dispatch(dst, src []byte) {
	if len(dst) < ntStoreThreshold {
		swizzleOpaqueAVX2(dst, src)
		done := len(dst) &^ 31
		swizzleOpaqueSWAR(dst[done:], src[done:])
		return
	}

	head := alignHead(dst)
	if head > 0 {
		swizzleOpaqueSWAR(dst[:head], src[:head])
	}
	body := (len(dst) - head) &^ 31
	swizzleOpaqueAVX2NT(dst[head:head+body], src[head:head+body])
	swizzleOpaqueSWAR(dst[head+body:], src[head+body:])
}

// alignHead returns the leading byte count that must be handled with unaligned
// stores for the remainder of dst to start on a 32-byte boundary. It returns
// len(dst) when no whole-pixel split can reach that boundary, which forces the
// caller onto the scalar path rather than misaligning a non-temporal store.
func alignHead(dst []byte) int {
	if len(dst) == 0 {
		return 0
	}
	offset := int(uintptr(unsafe.Pointer(&dst[0])) % vectorAlign)
	if offset == 0 {
		return 0
	}
	head := vectorAlign - offset
	if head%pixelSize != 0 || head > len(dst) {
		return len(dst)
	}
	return head
}
