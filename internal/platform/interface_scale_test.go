package platform

import (
	"encoding/json"
	"math"
	"testing"
)

func TestInterfaceScaleSettingsCompatibility(t *testing.T) {
	for _, scale := range []float64{0, 0.75, 1, 1.25, 2} {
		body, err := json.Marshal(AppSettings{InterfaceScale: scale})
		if err != nil {
			t.Fatal(err)
		}
		var loaded AppSettings
		if err := json.Unmarshal(body, &loaded); err != nil {
			t.Fatal(err)
		}
		normalizeAppSettingsShape(&loaded)
		if loaded.InterfaceScale != scale {
			t.Fatalf("scale %v did not survive save/load", scale)
		}
	}
	for _, scale := range []float64{-1, 0.5, 3, math.NaN(), math.Inf(1)} {
		loaded := AppSettings{InterfaceScale: scale}
		normalizeAppSettingsShape(&loaded)
		if loaded.InterfaceScale != 0 {
			t.Fatalf("invalid saved scale %v did not fall back to automatic", scale)
		}
		if err := (&PlatformService{}).SetInterfaceScale(scale); err == nil {
			t.Fatalf("invalid scale %v accepted", scale)
		}
	}
}

func TestInterfaceScalePersistsBeforeApplying(t *testing.T) {
	dir := testExeDirWithPortable(t)
	if err := SaveAppSettings(dir, AppSettings{Language: "de-DE"}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	SetInterfaceScaleHandler(func(scale float64) {
		calls++
		loaded, err := LoadAppSettings(dir)
		if err != nil || loaded.InterfaceScale != scale || loaded.Language != "de-DE" {
			t.Fatalf("scale applied before saving, or settings changed: %v %v", loaded, err)
		}
	})
	t.Cleanup(func() { SetInterfaceScaleHandler(nil) })
	svc := &PlatformService{}
	for _, value := range []float64{0.75, 1.5, 0} {
		if err := svc.SetInterfaceScale(value); err != nil {
			t.Fatal(err)
		}
		if got, err := svc.GetInterfaceScale(); err != nil || got != value {
			t.Fatalf("scale=%v err=%v", got, err)
		}
	}
	if calls != 3 {
		t.Fatalf("live updates=%d", calls)
	}
}
