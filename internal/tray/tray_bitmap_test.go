package tray

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"
)

func TestMenuBitmapRespectsManualAvatar(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("tray bitmaps are Windows-only")
	}
	paths.InitDataRoot(t.TempDir())
	dir, err := profileimage.ProfileDir("Steam")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var static bytes.Buffer
	if err := png.Encode(&static, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name       string
		id         string
		manual     bool
		primaryPNG bool
		wantBitmap bool
	}{
		{"Steam video uses static fallback", "76561190000000001", false, false, true},
		{"manual video ignores old static avatar", "76561190000000002", true, false, false},
		{"manual image remains visible", "76561190000000003", true, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			write := func(name string, data []byte) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			write(tt.id+"_static.png", static.Bytes())
			if tt.primaryPNG {
				write(tt.id+".png", static.Bytes())
			} else {
				// The tray image decoder cannot decode video files.
				write(tt.id+".webm", []byte{0x1a, 0x45, 0xdf, 0xa3})
			}
			if tt.manual {
				if err := profileimage.WriteManualProfileMarker("Steam", tt.id); err != nil {
					t.Fatal(err)
				}
			}
			got := menuBitmapForAccount("Steam", "+s:"+tt.id)
			if (len(got) > 0) != tt.wantBitmap {
				t.Fatalf("bitmap present = %v, want %v", len(got) > 0, tt.wantBitmap)
			}
		})
	}
}
