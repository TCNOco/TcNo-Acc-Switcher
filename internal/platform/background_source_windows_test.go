package platform

import "testing"

func TestResolveBackgroundSourcePathUsesMappedDriveUNC(t *testing.T) {
	got, ok := resolveBackgroundSourcePathWithLookup(`Y:\AI - Images\background.webp`, func(volume string) (string, bool) {
		if volume != `Y:` {
			t.Fatalf("mapped volume = %q, want Y:", volume)
		}
		return `\\server\media`, true
	})
	if !ok {
		t.Fatal("mapped drive path was not resolved")
	}
	want := `\\server\media\AI - Images\background.webp`
	if got != want {
		t.Fatalf("resolved path = %q, want %q", got, want)
	}
}

func TestResolveBackgroundSourcePathIgnoresLocalAndUNCSources(t *testing.T) {
	for _, path := range []string{`C:\images\background.webp`, `\\server\share\background.webp`, `background.webp`} {
		got, ok := resolveBackgroundSourcePathWithLookup(path, func(volume string) (string, bool) {
			if path != `C:\images\background.webp` || volume != `C:` {
				t.Fatalf("mapping lookup called for %q with volume %q", path, volume)
			}
			return "", false
		})
		if ok {
			t.Fatalf("resolveBackgroundSourcePath(%q) = %q, true; want unresolved", path, got)
		}
	}
}
