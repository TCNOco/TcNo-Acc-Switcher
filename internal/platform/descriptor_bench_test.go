package platform

import (
	"os"
	"path/filepath"
	"testing"
)

// benchPlatformsJSON is the real shipped catalog, so the parse cost measured
// here is the one the app actually pays.
func benchPlatformsJSON(tb testing.TB) []byte {
	tb.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "Platforms.json"))
	if err != nil {
		tb.Skipf("Platforms.json unavailable: %v", err)
	}
	return raw
}

// BenchmarkParseDescriptor is one descriptor lookup: a caller wanting a single
// platform's descriptor pays a full unmarshal of the whole catalog for it.
func BenchmarkParseDescriptor(b *testing.B) {
	raw := benchPlatformsJSON(b)
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ParseDescriptor(raw, "Steam"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseDescriptorAllPlatforms is the shape a sweep over every platform
// takes: one descriptor each, and so one full catalog unmarshal each.
func BenchmarkParseDescriptorAllPlatforms(b *testing.B) {
	raw := benchPlatformsJSON(b)
	names, err := parsePlatformNames(raw)
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("platforms in catalog: %d, catalog size: %d bytes", len(names), len(raw))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, n := range names {
			if _, err := ParseDescriptor(raw, n); err != nil {
				b.Fatal(err)
			}
		}
	}
}
