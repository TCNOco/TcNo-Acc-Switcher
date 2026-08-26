package qrrender

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/makiuchi-d/gozxing/qrcode/decoder"
)

func TestSVGDataURIDrawsTheChallengeURL(t *testing.T) {
	t.Parallel()

	uri, err := SVGDataURI("https://s.team/q/1/4123")
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "data:image/svg+xml;base64,"
	if !strings.HasPrefix(uri, prefix) {
		t.Fatalf("data URI = %.40q", uri)
	}
	svg, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, prefix))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(svg), "<svg") || !strings.Contains(string(svg), "viewBox") {
		t.Fatalf("payload is not an SVG: %.120q", svg)
	}
	// An empty fill resolves to the initial value - black - across the whole
	// background rect.
	if !strings.Contains(string(svg), `fill="#FFFFFF"`) {
		t.Fatal("the quiet zone was left without a colour to paint it")
	}
	if strings.Contains(string(svg), `fill=""`) {
		t.Fatal("an empty fill reached the page, which paints black")
	}
}

func TestSVGDataURIRefusesNothingToDraw(t *testing.T) {
	t.Parallel()

	for name, text := range map[string]string{
		"empty":      "",
		"whitespace": "   ",
		"oversized":  strings.Repeat("a", maxTextBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := SVGDataURI(text); err == nil {
				t.Fatal("a code was drawn for nothing")
			}
		})
	}
}

// The path is written by hand, so the geometry is this package's to get wrong.
func TestSVGDataURIDrawsEveryModuleAndNothingElse(t *testing.T) {
	t.Parallel()

	const text = "https://s.team/q/1/1234567890123456789"
	matrix, err := qrcode.NewQRCodeWriter().Encode(text, gozxing.BarcodeFormat_QR_CODE, 0, 0,
		map[gozxing.EncodeHintType]interface{}{
			gozxing.EncodeHintType_ERROR_CORRECTION: decoder.ErrorCorrectionLevel_M,
			gozxing.EncodeHintType_MARGIN:           0,
		})
	if err != nil {
		t.Fatal(err)
	}
	dark := 0
	for y := 0; y < matrix.GetHeight(); y++ {
		for x := 0; x < matrix.GetWidth(); x++ {
			if matrix.Get(x, y) {
				dark++
			}
		}
	}

	uri, err := SVGDataURI(text)
	if err != nil {
		t.Fatal(err)
	}
	svg, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, "data:image/svg+xml;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(svg), "M"); got != dark {
		t.Fatalf("drew %d modules, symbol has %d", got, dark)
	}
	want := fmt.Sprintf(`viewBox="0 0 %d %d"`, (matrix.GetWidth()+border*2)*scale, (matrix.GetHeight()+border*2)*scale)
	if !strings.Contains(string(svg), want) {
		t.Fatalf("viewBox does not leave the quiet zone room; wanted %s", want)
	}
}
