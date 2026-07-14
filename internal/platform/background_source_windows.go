package platform

import (
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	mprDLL                = windows.NewLazySystemDLL("mpr.dll")
	wNetGetConnectionProc = mprDLL.NewProc("WNetGetConnectionW")
)

func resolveBackgroundSourcePath(path string) (string, bool) {
	return resolveBackgroundSourcePathWithLookup(path, mappedDriveRemote)
}

func resolveBackgroundSourcePathWithLookup(path string, lookup func(string) (string, bool)) (string, bool) {
	path = filepath.Clean(path)
	volume := filepath.VolumeName(path)
	if len(volume) != 2 || volume[1] != ':' || !filepath.IsAbs(path) {
		return "", false
	}

	remote, ok := lookup(volume)
	if !ok {
		return "", false
	}
	remainder := strings.TrimLeft(path[len(volume):], `\/`)
	if remainder == "" {
		return filepath.Clean(remote), true
	}
	return filepath.Join(remote, remainder), true
}

func mappedDriveRemote(volume string) (string, bool) {
	if remote, ok := mappedDriveRemoteFromMPR(volume); ok {
		return remote, true
	}

	drive := strings.ToUpper(strings.TrimSuffix(volume, ":"))
	key, err := registry.OpenKey(registry.CURRENT_USER, `Network\`+drive, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer key.Close()
	remote, _, err := key.GetStringValue("RemotePath")
	remote = strings.TrimSpace(remote)
	return remote, err == nil && remote != ""
}

func mappedDriveRemoteFromMPR(volume string) (string, bool) {
	localName, err := windows.UTF16PtrFromString(volume)
	if err != nil {
		return "", false
	}

	size := uint32(512)
	for range 2 {
		buffer := make([]uint16, size)
		status, _, _ := wNetGetConnectionProc.Call(
			uintptr(unsafe.Pointer(localName)),
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(unsafe.Pointer(&size)),
		)
		switch syscallError := windows.Errno(status); syscallError {
		case windows.ERROR_SUCCESS:
			remote := strings.TrimSpace(windows.UTF16ToString(buffer))
			return remote, remote != ""
		case windows.ERROR_MORE_DATA:
			continue
		default:
			return "", false
		}
	}
	return "", false
}
