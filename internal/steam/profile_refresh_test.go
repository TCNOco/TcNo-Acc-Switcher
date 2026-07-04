package steam

import (
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"
)

func TestRefreshAllSteamImagesClearsProfileMetadataCaches(t *testing.T) {
	paths.ResetForTest(t.TempDir())

	const steamID = "76561198000000000"

	xmlPath, err := xmlCachePath(steamID)
	if err != nil {
		t.Fatalf("xmlCachePath: %v", err)
	}
	miniPath, err := miniprofileCachePath(steamID)
	if err != nil {
		t.Fatalf("miniprofileCachePath: %v", err)
	}
	vacPath, err := vacCachePath()
	if err != nil {
		t.Fatalf("vacCachePath: %v", err)
	}
	profileDir, err := profileimage.ProfileDir(PlatformKey)
	if err != nil {
		t.Fatalf("ProfileDir: %v", err)
	}
	imagePath := filepath.Join(profileDir, steamID+".webp")

	writeTestFile(t, xmlPath, "<profile><steamID>Old Name</steamID></profile>")
	writeTestFile(t, miniPath, `<span class="persona">Old Mini</span>`)
	writeTestFile(t, vacPath, `[{"SteamID":"`+steamID+`","Vac":false,"Ltd":false}]`)
	writeTestFile(t, imagePath, "old image")

	svc := NewSteamService()
	if err := svc.RefreshAllSteamImages(); err != nil {
		t.Fatalf("RefreshAllSteamImages: %v", err)
	}
	stopRefreshTimer(svc)

	assertMissing(t, xmlPath)
	assertMissing(t, miniPath)
	assertMissing(t, vacPath)
	assertMissing(t, imagePath)
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, stat err=%v", path, err)
	}
}

func stopRefreshTimer(svc *SteamService) {
	svc.refreshMu.Lock()
	defer svc.refreshMu.Unlock()
	if svc.refreshTimer != nil {
		svc.refreshTimer.Stop()
		svc.refreshTimer = nil
	}
}
