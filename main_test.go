package main

import (
	"bytes"
	"image"
	_ "image/png"
	"testing"
)

func TestTrayIconPNGIsTraySized(t *testing.T) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(trayIconPNG))
	if err != nil {
		t.Fatalf("decode tray icon: %v", err)
	}
	if format != "png" {
		t.Fatalf("tray icon format = %q, want png", format)
	}
	if cfg.Width != 32 || cfg.Height != 32 {
		t.Fatalf("tray icon dimensions = %dx%d, want 32x32", cfg.Width, cfg.Height)
	}
}
