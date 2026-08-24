package qrrender

import (
	"encoding/base64"
	"strings"
	"testing"
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
	// A transparent light colour is written as fill="", which browsers resolve to
	// the initial value - black - across the whole background rect, so the code
	// arrived as a solid black square.
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
