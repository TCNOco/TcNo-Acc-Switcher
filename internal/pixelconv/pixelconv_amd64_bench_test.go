//go:build amd64

package pixelconv

import (
	"fmt"
	"testing"
)

func BenchmarkOpaqueKernelsAMD64(b *testing.B) {
	kernels := []struct {
		name    string
		fn      func(dst, src []byte)
		needs   bool
		aligned bool
	}{
		{name: "SSSE3", fn: swizzleOpaqueSSSE3, needs: hasSSSE3},
		{name: "AVX2", fn: swizzleOpaqueAVX2, needs: hasAVX2},
		{name: "AVX2NT", fn: swizzleOpaqueAVX2NT, needs: hasAVX2, aligned: true},
	}
	for _, size := range benchSizes {
		for _, kernel := range kernels {
			if !kernel.needs {
				continue
			}
			b.Run(fmt.Sprintf("%s/%s", size.name, kernel.name), func(b *testing.B) {
				n := size.pixels * pixelSize
				src := randomBGRA(b, n)
				dst := make([]byte, n)
				if kernel.aligned {
					dst = alignedBuffer(b, n)
				}
				b.SetBytes(int64(n))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					kernel.fn(dst, src)
				}
			})
		}
	}
}
