//go:build windows

package winutil

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

func propVariantVT(pv *propVariant) uint16 {
	return *(*uint16)(unsafe.Pointer(&pv.data[0]))
}

func propVariantValue(pv *propVariant) *uint16 {
	return *(**uint16)(unsafe.Pointer(&pv.data[propVariantValueOffset]))
}

// The AppUserModelID path binds Windows procs by name. LazyProc.Call panics
// rather than erroring when a name is missing, so a wrong name is a crash at
// the point of use, not a build failure. propsys.dll!InitPropVariantFromString
// is an inline helper in propvarutil.h, not a dependable export. These tests
// call the real procs so a bad binding fails here instead of in front of a user.
func TestInitPropVariantFromStringRoundTrips(t *testing.T) {
	pv, err := initPropVariantFromString("TcNo.Test.AppID")
	if err != nil {
		t.Fatalf("initPropVariantFromString: %v", err)
	}
	defer clearPropVariant(&pv)

	if got := propVariantVT(&pv); got != vtLPWSTR {
		t.Errorf("vt = %d, want VT_LPWSTR (%d)", got, vtLPWSTR)
	}
	if propVariantValue(&pv) == nil {
		t.Error("string pointer is nil; the allocation did not happen")
	}
}

func TestClearPropVariantIsSafeToRepeat(t *testing.T) {
	pv, err := initPropVariantFromString("TcNo.Test.AppID")
	if err != nil {
		t.Fatalf("initPropVariantFromString: %v", err)
	}
	clearPropVariant(&pv)
	if propVariantValue(&pv) != nil {
		t.Error("clear left a dangling pointer, so a second clear would double-free")
	}
	clearPropVariant(&pv)
}

// Exercises the whole chain against a real .lnk: SHGetPropertyStoreFromParsingName,
// SetValue and Commit all bind by name too.
func TestSetShortcutAppUserModelIDOnARealShortcut(t *testing.T) {
	dir := t.TempDir()
	lnk := filepath.Join(dir, "tcno-appid-test.lnk")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteShortcutLnk(lnk, exe, "", dir, "test", "", ShortcutAppUserModelID("test", "appid")); err != nil {
		t.Fatalf("WriteShortcutLnk: %v", err)
	}
	if _, err := os.Stat(lnk); err != nil {
		t.Fatalf("shortcut was not written: %v", err)
	}
	if err := setShortcutAppUserModelID(lnk, "TcNo.Test.Explicit"); err != nil {
		t.Errorf("setShortcutAppUserModelID: %v", err)
	}
}
