// Package pixelconv converts between the 32-bit pixel layouts the app moves
// between Windows GDI and Go's image package.
//
// Windows hands back BGRA; image.NRGBA wants RGBA. That swap is a byte shuffle
// over buffers that reach tens of megabytes (a full-screen QR capture is capped
// at 24 megapixels, so 96 MB in and 96 MB out), which makes it worth a vector
// kernel. On amd64 the work is done by PSHUFB/VPSHUFB; everywhere else a SWAR
// fallback moves eight bytes per iteration.
package pixelconv

import "encoding/binary"

// pixelSize is the width of one pixel in both the source and destination
// layouts. Every conversion is truncated to a whole number of pixels.
const pixelSize = 4

const (
	// alphaMask64 sets both alpha bytes of a two-pixel word to 0xff.
	alphaMask64 = 0xFF000000FF000000
	// laneMask64 selects the low byte of each pixel in a two-pixel word.
	laneMask64 = 0x000000FF000000FF
	// greenMask64 selects the green byte of each pixel in a two-pixel word.
	greenMask64 = 0x0000FF000000FF00
	// alphaKeep64 selects the alpha byte of each pixel in a two-pixel word.
	alphaKeep64 = 0xFF000000FF000000
)

// BGRAToNRGBAOpaque writes min(len(dst), len(src)) bytes, truncated to whole
// pixels, converting BGRA to NRGBA and forcing every output alpha to 0xff.
// dst and src must not overlap partially; identical slices are safe.
func BGRAToNRGBAOpaque(dst, src []byte) {
	n := convertible(dst, src)
	if n == 0 {
		return
	}
	swizzleOpaque(dst[:n], src[:n])
}

// BGRAToNRGBA is BGRAToNRGBAOpaque but carries the source alpha through
// unchanged.
func BGRAToNRGBA(dst, src []byte) {
	n := convertible(dst, src)
	if n == 0 {
		return
	}
	swizzleKeepAlpha(dst[:n], src[:n])
}

func convertible(dst, src []byte) int {
	n := len(src)
	if len(dst) < n {
		n = len(dst)
	}
	return n &^ (pixelSize - 1)
}

// swizzleOpaqueSWAR is the portable kernel and the reference the amd64 kernels
// are tested against. n is a whole number of pixels.
func swizzleOpaqueSWAR(dst, src []byte) {
	n := len(src)
	i := 0
	for ; i+8 <= n; i += 8 {
		v := binary.LittleEndian.Uint64(src[i : i+8])
		out := alphaMask64 |
			(v>>16)&laneMask64 |
			v&greenMask64 |
			(v&laneMask64)<<16
		binary.LittleEndian.PutUint64(dst[i:i+8], out)
	}
	for ; i+4 <= n; i += 4 {
		v := binary.LittleEndian.Uint32(src[i : i+4])
		out := 0xFF000000 |
			(v>>16)&0x000000FF |
			v&0x0000FF00 |
			(v&0x000000FF)<<16
		binary.LittleEndian.PutUint32(dst[i:i+4], out)
	}
}

func swizzleKeepAlphaSWAR(dst, src []byte) {
	n := len(src)
	i := 0
	for ; i+8 <= n; i += 8 {
		v := binary.LittleEndian.Uint64(src[i : i+8])
		out := v&alphaKeep64 |
			(v>>16)&laneMask64 |
			v&greenMask64 |
			(v&laneMask64)<<16
		binary.LittleEndian.PutUint64(dst[i:i+8], out)
	}
	for ; i+4 <= n; i += 4 {
		v := binary.LittleEndian.Uint32(src[i : i+4])
		out := v&0xFF000000 |
			(v>>16)&0x000000FF |
			v&0x0000FF00 |
			(v&0x000000FF)<<16
		binary.LittleEndian.PutUint32(dst[i:i+4], out)
	}
}

// swizzleOpaqueScalar is the byte-at-a-time form this package replaced. It is
// retained as the benchmark baseline and as an independent oracle in tests.
func swizzleOpaqueScalar(dst, src []byte) {
	for i := 0; i+4 <= len(src); i += 4 {
		dst[i] = src[i+2]
		dst[i+1] = src[i+1]
		dst[i+2] = src[i]
		dst[i+3] = 0xff
	}
}

func swizzleKeepAlphaScalar(dst, src []byte) {
	for i := 0; i+4 <= len(src); i += 4 {
		dst[i] = src[i+2]
		dst[i+1] = src[i+1]
		dst[i+2] = src[i]
		dst[i+3] = src[i+3]
	}
}
