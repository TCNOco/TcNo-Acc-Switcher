//go:build windows

package qrcapture

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	th32csSnapProcess        = 0x00000002
	processQueryLimitedInfo  = 0x1000
	gwOwner                  = 4
	dwmwaExtendedFrameBounds = 9
	dwmwaCloaked             = 14
)

var (
	user32QR                   = windows.NewLazySystemDLL("user32.dll")
	dwmapiQR                   = windows.NewLazySystemDLL("dwmapi.dll")
	procEnumWindows            = user32QR.NewProc("EnumWindows")
	procIsWindowVisible        = user32QR.NewProc("IsWindowVisible")
	procGetWindow              = user32QR.NewProc("GetWindow")
	procGetWindowThreadProcess = user32QR.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW         = user32QR.NewProc("GetWindowTextW")
	procGetClassNameW          = user32QR.NewProc("GetClassNameW")
	procEnumDisplayMonitors    = user32QR.NewProc("EnumDisplayMonitors")
	procDwmGetWindowAttribute  = dwmapiQR.NewProc("DwmGetWindowAttribute")
	enumWindowsLock            sync.Mutex
	enumWindowsDestination     *[]WindowInfo
	enumWindowsCallbackError   error
	enumWindowsCallback        = syscall.NewCallback(collectWindow)
	enumMonitorsLock           sync.Mutex
	enumMonitorsDestination    *[]Rect
	enumMonitorsCallback       = syscall.NewCallback(collectMonitor)
)

type windowsBackend struct{}

type nativeRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

func newBackend() Backend { return windowsBackend{} }

func (windowsBackend) RegistrySteamPaths() ([]string, error) {
	queries := []struct {
		root   registry.Key
		key    string
		values []string
	}{
		{registry.CURRENT_USER, `Software\Valve\Steam`, []string{"SteamPath", "SteamExe"}},
		{registry.LOCAL_MACHINE, `SOFTWARE\Valve\Steam`, []string{"InstallPath"}},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, []string{"InstallPath"}},
	}
	seen := make(map[string]struct{})
	var paths []string
	for _, query := range queries {
		key, err := registry.OpenKey(query.root, query.key, registry.QUERY_VALUE|registry.WOW64_64KEY)
		if err != nil {
			key, err = registry.OpenKey(query.root, query.key, registry.QUERY_VALUE|registry.WOW64_32KEY)
		}
		if err != nil {
			continue
		}
		for _, valueName := range query.values {
			value, _, err := key.GetStringValue(valueName)
			if err != nil || strings.TrimSpace(value) == "" {
				continue
			}
			value = os.ExpandEnv(strings.TrimSpace(value))
			lookup := strings.ToLower(filepath.Clean(value))
			if _, exists := seen[lookup]; exists {
				continue
			}
			seen[lookup] = struct{}{}
			paths = append(paths, value)
		}
		key.Close()
	}
	return paths, nil
}

func (windowsBackend) RunningProcesses() ([]ProcessInfo, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(th32csSnapProcess, 0)
	if err != nil {
		return nil, errors.Join(ErrPlatform, err)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, errors.Join(ErrPlatform, err)
	}
	var processes []ProcessInfo
	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if isSteamExecutable(name) {
			if path, ok := queryProcessPath(entry.ProcessID); ok {
				processes = append(processes, ProcessInfo{PID: entry.ProcessID, ExecutablePath: path})
			}
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if errors.Is(err, syscall.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, errors.Join(ErrPlatform, err)
		}
	}
	return processes, nil
}

func queryProcessPath(pid uint32) (string, bool) {
	handle, err := windows.OpenProcess(processQueryLimitedInfo, false, pid)
	if err != nil {
		return "", false
	}
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil || size == 0 || size >= uint32(len(buffer)) {
		return "", false
	}
	return windows.UTF16ToString(buffer[:size]), true
}

func (windowsBackend) CanonicalSteamRoot(hint string) (string, error) {
	path := strings.TrimSpace(os.ExpandEnv(hint))
	if path == "" {
		return "", fs.ErrInvalid
	}
	path = filepath.Clean(path)
	if isSteamExecutable(path) {
		path = filepath.Dir(path)
	}
	for depth := 0; depth < 10; depth++ {
		root, err := canonicalDirectory(path)
		if err == nil {
			steamExe := filepath.Join(root, "steam.exe")
			if executable, exeErr := canonicalFile(steamExe); exeErr == nil && strings.EqualFold(filepath.Dir(executable), root) {
				return root, nil
			}
		}
		parent := filepath.Dir(path)
		if samePath(parent, path) {
			break
		}
		path = parent
	}
	return "", fs.ErrNotExist
}

func (windowsBackend) CanonicalExecutable(path string) (string, error) {
	canonical, err := canonicalFile(path)
	if err != nil {
		return "", err
	}
	if !isSteamExecutable(canonical) {
		return "", fs.ErrInvalid
	}
	return canonical, nil
}

func canonicalDirectory(path string) (string, error) {
	return canonicalPath(path, true)
}

func canonicalFile(path string) (string, error) {
	return canonicalPath(path, false)
}

func canonicalPath(path string, directory bool) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	if err := rejectReparseComponents(absolute); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	resolved, err = filepath.Abs(filepath.Clean(resolved))
	if err != nil || !samePath(absolute, resolved) {
		return "", fs.ErrPermission
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if directory != info.IsDir() || (!directory && !info.Mode().IsRegular()) {
		return "", fs.ErrInvalid
	}
	return resolved, nil
}

func rejectReparseComponents(path string) error {
	volume := filepath.VolumeName(path)
	if volume == "" {
		return fs.ErrInvalid
	}
	current := volume + string(filepath.Separator)
	remainder := strings.TrimPrefix(path, current)
	for _, component := range strings.Split(remainder, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		pointer, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return err
		}
		attributes, err := windows.GetFileAttributes(pointer)
		if err != nil {
			return err
		}
		if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fs.ErrPermission
		}
	}
	return nil
}

func (windowsBackend) TopLevelWindows() ([]WindowInfo, error) {
	enumWindowsLock.Lock()
	defer enumWindowsLock.Unlock()
	var windowsFound []WindowInfo
	enumWindowsDestination = &windowsFound
	enumWindowsCallbackError = nil
	defer func() { enumWindowsDestination = nil }()
	result, _, callErr := procEnumWindows.Call(enumWindowsCallback, 0)
	if enumWindowsCallbackError != nil {
		return nil, enumWindowsCallbackError
	}
	if result == 0 {
		return nil, wrapWindowsCall(callErr)
	}
	return windowsFound, nil
}

func collectWindow(hwnd uintptr, _ uintptr) uintptr {
	if enumWindowsDestination == nil || hwnd == 0 {
		return 1
	}
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	var pid uint32
	procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	owner, _, _ := procGetWindow.Call(hwnd, gwOwner)
	var cloaked uint32
	result, _, callErr := procDwmGetWindowAttribute.Call(hwnd, dwmwaCloaked, uintptr(unsafe.Pointer(&cloaked)), unsafe.Sizeof(cloaked))
	if int32(result) < 0 {
		_ = callErr
		return 1
	}
	var bounds nativeRect
	result, _, callErr = procDwmGetWindowAttribute.Call(hwnd, dwmwaExtendedFrameBounds, uintptr(unsafe.Pointer(&bounds)), unsafe.Sizeof(bounds))
	if int32(result) < 0 {
		_ = callErr
		return 1
	}
	*enumWindowsDestination = append(*enumWindowsDestination, WindowInfo{
		Handle: hwnd, Owner: owner, PID: pid,
		Title: windowText(hwnd), ClassName: windowClass(hwnd),
		Visible: visible != 0, Cloaked: cloaked != 0,
		Bounds: Rect{Left: bounds.Left, Top: bounds.Top, Right: bounds.Right, Bottom: bounds.Bottom},
	})
	return 1
}

func windowText(hwnd uintptr) string {
	var buffer [1024]uint16
	length, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if length == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer[:minInt(int(length), len(buffer)-1)])
}

func windowClass(hwnd uintptr) string {
	var buffer [256]uint16
	length, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if length == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer[:minInt(int(length), len(buffer)-1)])
}

func (windowsBackend) MonitorBounds() ([]Rect, error) {
	enumMonitorsLock.Lock()
	defer enumMonitorsLock.Unlock()
	var monitors []Rect
	enumMonitorsDestination = &monitors
	defer func() { enumMonitorsDestination = nil }()
	result, _, callErr := procEnumDisplayMonitors.Call(0, 0, enumMonitorsCallback, 0)
	if result == 0 {
		return nil, wrapWindowsCall(callErr)
	}
	return monitors, nil
}

func collectMonitor(_ uintptr, _ uintptr, rectAddress uintptr, _ uintptr) uintptr {
	if enumMonitorsDestination == nil || rectAddress == 0 {
		return 1
	}
	rect := (*nativeRect)(unsafe.Add(unsafe.Pointer(nil), rectAddress))
	*enumMonitorsDestination = append(*enumMonitorsDestination, Rect{
		Left: rect.Left, Top: rect.Top, Right: rect.Right, Bottom: rect.Bottom,
	})
	return 1
}

func wrapWindowsCall(callErr error) error {
	if callErr == nil || errors.Is(callErr, syscall.Errno(0)) {
		return ErrPlatform
	}
	return errors.Join(ErrPlatform, callErr)
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
