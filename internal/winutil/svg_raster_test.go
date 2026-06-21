package winutil

import (
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestApplyPlatformSVGTheming_pathIdFGNotFirst(t *testing.T) {
	// Mirrors Delta Force Hawk Ops: id="FG" after d=.
	const in = `<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%" viewBox="0 0 500 500"><path d="M10 10H490V490H10Z" id="FG"/></svg>`
	out := string(ApplyPlatformSVGTheming([]byte(in), "#23272A", "#00ff00"))
	if out == in {
		t.Fatal("expected SVG to be modified")
	}
	for _, sub := range []string{`width="500"`, `height="500"`, `<rect fill="#23272A"`, `fill="#00ff00"`, `id="FG"`} {
		if !strings.Contains(out, sub) {
			t.Fatalf("missing %q in:\n%s", sub, out)
		}
	}
}

func TestRasterizeSVGOK_percentSizeSquare(t *testing.T) {
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%" viewBox="0 0 100 100"><path id="FG" d="M10 10H90V90H10Z"/></svg>`
	themed := ApplyPlatformSVGTheming([]byte(svg), "#23272A", "#80ffea")
	img, err := rasterizeSVGOK(themed, 100)
	if err != nil {
		t.Fatal(err)
	}
	// Center should be foreground (accent), not background grey.
	c := img.At(50, 50).(color.NRGBA)
	if c.R == 0x23 && c.G == 0x27 && c.B == 0x2A {
		t.Fatalf("center pixel still background grey, got %#v", c)
	}
}

func TestRasterizeSVGOK_realPlatformBattleNet(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	p := filepath.Join(repo, "frontend", "public", "img", "platform", "BattleNet.svg")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("platform svg not at %s: %v", p, err)
	}
	themed := ApplyPlatformSVGTheming(b, DefaultPlatformLogoBackground, DefaultPlatformLogoForeground)
	img, err := rasterizeSVGOK(themed, 256)
	if err != nil {
		t.Fatal(err)
	}
	c := img.At(128, 128).(color.NRGBA)
	if c.R == 0x23 && c.G == 0x27 && c.B == 0x2A {
		t.Fatalf("center-ish pixel still flat background only, got %#v", c)
	}
}
