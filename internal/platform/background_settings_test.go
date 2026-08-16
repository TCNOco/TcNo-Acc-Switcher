package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppSettingsAddsBackgroundLayoutToOldJSON(t *testing.T) {
	setTestAppData(t)
	exeDir := filepath.Join(t.TempDir(), "bin")
	settingsDir := PortableUserDataDir(exeDir)
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{
		"version": 1,
		"language": "en-US",
		"platformBgs": {
			"Steam": {"image": "platform-Steam-bg.webp"}
		}
	}`)
	if err := os.WriteFile(filepath.Join(settingsDir, settingsFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	ResetPathSingletonsForTest(exeDir)
	settings, err := LoadAppSettings(exeDir)
	if err != nil {
		t.Fatal(err)
	}

	if settings.AppBgAlignment != defaultBgAlignment || settings.AppBgFit != defaultBgFit {
		t.Fatalf("app background layout = %q/%q, want %q/%q", settings.AppBgAlignment, settings.AppBgFit, defaultBgAlignment, defaultBgFit)
	}
	steam := settings.PlatformBgs["Steam"]
	if steam.Alignment != defaultBgAlignment || steam.Fit != defaultBgFit {
		t.Fatalf("Steam background layout = %q/%q, want %q/%q", steam.Alignment, steam.Fit, defaultBgAlignment, defaultBgFit)
	}
}

func TestNormalizeAppSettingsDefaultsSanitizesBackgroundLayout(t *testing.T) {
	settings := AppSettings{
		AppBgAlignment: "outside",
		AppBgFit:       "stretch-more",
		PlatformBgs: map[string]PlatformBgSettings{
			"Steam": {Alignment: " diagonal ", Fit: "crop"},
			"Epic":  {Alignment: " RIGHT ", Fit: " SCALE-DOWN "},
		},
	}

	normalizeAppSettingsDefaults(&settings, map[string]json.RawMessage{})

	if settings.AppBgAlignment != defaultBgAlignment || settings.AppBgFit != defaultBgFit {
		t.Fatalf("invalid app layout normalized to %q/%q, want %q/%q", settings.AppBgAlignment, settings.AppBgFit, defaultBgAlignment, defaultBgFit)
	}
	steam := settings.PlatformBgs["Steam"]
	if steam.Alignment != defaultBgAlignment || steam.Fit != defaultBgFit {
		t.Fatalf("invalid platform layout normalized to %q/%q, want %q/%q", steam.Alignment, steam.Fit, defaultBgAlignment, defaultBgFit)
	}
	epic := settings.PlatformBgs["Epic"]
	if epic.Alignment != "right" || epic.Fit != "scale-down" {
		t.Fatalf("valid platform layout normalized to %q/%q, want right/scale-down", epic.Alignment, epic.Fit)
	}
}

func TestBuildAppBgInfoAlwaysReturnsNormalizedBackgroundLayout(t *testing.T) {
	info := buildAppBgInfo("", 0, 0, "Top", "contain", false, BackgroundLuma{})
	if info.Alignment != "top" || info.Fit != "contain" {
		t.Fatalf("background info layout = %q/%q, want top/contain", info.Alignment, info.Fit)
	}

	info = buildAppBgInfo("app-bg.webp", 0.5, 2, "diagonal", "crop", true, BackgroundLuma{})
	if info.Alignment != defaultBgAlignment || info.Fit != defaultBgFit {
		t.Fatalf("invalid background info layout = %q/%q, want %q/%q", info.Alignment, info.Fit, defaultBgAlignment, defaultBgFit)
	}
}

func TestBuildAppBgInfoCarriesMeasuredLuma(t *testing.T) {
	luma := BackgroundLuma{Measured: true, Mean: 0.12, Low: 0.04, High: 0.31}
	info := buildAppBgInfo("app-bg.png", 0.5, 2, "top", "cover", true, luma)
	if info.Luma != luma {
		t.Fatalf("luma = %+v, want %+v", info.Luma, luma)
	}

	// No image means no background to measure, so the luma must not leak through
	// and convince the UI it knows how bright a picture that is not showing is.
	if info := buildAppBgInfo("", 0, 0, "top", "cover", false, luma); info.Luma.Measured {
		t.Fatalf("luma = %+v, want unmeasured when there is no image", info.Luma)
	}
}
