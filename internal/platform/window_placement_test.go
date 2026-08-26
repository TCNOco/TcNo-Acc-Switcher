package platform

import (
	"errors"
	"testing"
)

func TestSaveWindowPlacementRoundTrip(t *testing.T) {
	dir := testExeDirWithPortable(t)
	svc := &PlatformService{}

	want := WindowPlacement{Width: 1280, Height: 800, X: 0, Y: 0, Maximised: true}
	if err := SaveWindowPlacement(svc, want); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadAppSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	// X and Y of zero are a real position, so they have to survive the omitempty
	// tags that drop them from the file.
	if got := SavedWindowPlacement(loaded); got != want {
		t.Fatalf("placement = %+v, want %+v", got, want)
	}
}

func TestSaveWindowPlacementLeavesOtherSettingsAlone(t *testing.T) {
	dir := testExeDirWithPortable(t)
	svc := &PlatformService{}
	if err := SaveAppSettings(dir, AppSettings{Version: 1, Language: "de-DE", ExitToTray: true}); err != nil {
		t.Fatal(err)
	}

	if err := SaveWindowPlacement(svc, WindowPlacement{Width: 900, Height: 700}); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadAppSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Language != "de-DE" || !loaded.ExitToTray {
		t.Fatalf("unrelated settings changed: %+v", loaded)
	}
}

func TestSaveWindowPlacementWithoutService(t *testing.T) {
	if err := SaveWindowPlacement(nil, WindowPlacement{Width: 900, Height: 700}); !errors.Is(err, ErrNoSettingsService) {
		t.Fatalf("err = %v, want %v", err, ErrNoSettingsService)
	}
}

func TestWindowPlacementHasSize(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    WindowPlacement
		want bool
	}{
		{"unset", WindowPlacement{}, false},
		{"origin with size", WindowPlacement{Width: 800, Height: 600}, true},
		{"maximised without size", WindowPlacement{Maximised: true}, false},
		{"height only", WindowPlacement{Height: 600}, false},
	} {
		if got := tc.p.HasSize(); got != tc.want {
			t.Errorf("%s: HasSize() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
