//go:build windows

package platforms

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestReadCacheWhileOpen(t *testing.T) {
	path := filepath.Join("testdata", "ea_desktop", "CEF", "2", "EADesktop", "Default", "Cache", "Cache_Data", "data_1")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	// Reproduce Chromium's open handle without modifying the fixture.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.DELETE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	if _, err := os.ReadFile(path); err == nil {
		t.Fatal("expected normal file reading to fail with Chromium's sharing mode")
	}
	got, err := readCacheFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("shared cache read = %q, %v; want %q", got, err, want)
	}
}
