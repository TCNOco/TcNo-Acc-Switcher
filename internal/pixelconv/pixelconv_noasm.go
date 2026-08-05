//go:build !amd64

package pixelconv

func swizzleOpaque(dst, src []byte) { swizzleOpaqueSWAR(dst, src) }

func swizzleKeepAlpha(dst, src []byte) { swizzleKeepAlphaSWAR(dst, src) }
