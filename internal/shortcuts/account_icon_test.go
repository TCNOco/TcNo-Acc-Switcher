package shortcuts

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/winutil"
)

func TestAccountShortcutIcons(t *testing.T) {
	root := t.TempDir()
	paths.ResetForTest(root)
	var avatar bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 255, 255
	}
	if err := png.Encode(&avatar, img); err != nil {
		t.Fatal(err)
	}
	// A cached platform icon keeps this test independent of frontend assets.
	var platformICO bytes.Buffer
	if err := winutil.SavePNGsAsICO(&platformICO, [][]byte{avatar.Bytes()}); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(root, "IconCache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"steam", "discord"} {
		if err := os.WriteFile(filepath.Join(cache, platform+"_platform.ico"), platformICO.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, platform, primary, want string
		static, manual                bool
	}{
		{name: "static-only", platform: "Steam", static: true, want: "_static.png"},
		{name: "primary", platform: "Steam", primary: ".png", static: true, want: ".png"},
		{name: "video", platform: "Steam", primary: ".webm", static: true, want: "_static.png"},
		{name: "corrupt", platform: "Steam", primary: ".jpg", static: true, want: "_static.png"},
		{name: "manual-image", platform: "Steam", primary: ".png", static: true, manual: true, want: ".png"},
		{name: "manual-video", platform: "Steam", primary: ".webm", static: true, manual: true},
		{name: "no-avatar", platform: "Steam"},
		{name: "other-platform", platform: "Discord", static: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := profileimage.ProfileDir(tc.platform)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			write := func(suffix string, data []byte) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, tc.name+suffix), data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.primary != "" {
				data := []byte("not a still image")
				if tc.primary == ".png" {
					data = avatar.Bytes()
				}
				write(tc.primary, data)
			}
			if tc.static {
				write("_static.png", avatar.Bytes())
			}
			if tc.manual {
				write(profileimage.ManualProfileMarkerSuffix, nil)
			}
			got, ok := cachedShortcutAvatar(tc.platform, tc.name)
			if ok != (tc.want != "") || ok && filepath.Base(got) != tc.name+tc.want {
				t.Fatalf("avatar = %q, %v; want suffix %q", got, ok, tc.want)
			}
			icon := buildAccountShortcutIcon(tc.platform, tc.name)
			wantStem := profileimage.PlatformFolder(tc.platform) + "_platform.ico"
			if tc.want != "" {
				wantStem = profileimage.PlatformFolder(tc.platform) + "_" + tc.name + ".ico"
			}
			if icon != filepath.Join(cache, wantStem)+",0" {
				t.Fatalf("icon = %q, want %s", icon, wantStem)
			}
			data, err := os.ReadFile(strings.TrimSuffix(icon, ",0"))
			if err != nil || len(data) < 22 || binary.LittleEndian.Uint16(data[2:4]) != 1 {
				t.Fatalf("not a shell ICO: %v", err)
			}
			offset := binary.LittleEndian.Uint32(data[18:22])
			decoded, err := png.Decode(bytes.NewReader(data[offset:]))
			if err != nil {
				t.Fatal(err)
			}
			if got := color.NRGBAModel.Convert(decoded.At(decoded.Bounds().Max.X-1, decoded.Bounds().Max.Y-1)).(color.NRGBA); got.R != 255 || got.A != 255 {
				t.Fatalf("icon lost the avatar pixels: %v", got)
			}
			if gameIcon := buildGameAccountShortcutIcon(tc.platform, tc.name, "missing-game.url"); gameIcon != icon {
				t.Fatalf("game shortcut fallback = %q, want %q", gameIcon, icon)
			}
		})
	}
}
