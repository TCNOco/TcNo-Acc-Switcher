//go:build windows

package winutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// WriteShortcutLnk creates a Windows shell shortcut via WScript.Shell COM.
// appUserModelID, when set, gives the shortcut a unique Start Menu / pinned-tile identity.
func WriteShortcutLnk(shortcutPath, targetExe, arguments, workingDir, description, iconLocation, appUserModelID string) error {
	shortcutPath = filepath.Clean(shortcutPath)
	targetExe = strings.TrimSpace(targetExe)
	if shortcutPath == "" || targetExe == "" {
		return fmt.Errorf("shortcut path or target empty")
	}
	if err := os.MkdirAll(filepath.Dir(shortcutPath), 0o755); err != nil {
		return err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := writeShortcutLnkCOM(shortcutPath, targetExe, arguments, workingDir, description, iconLocation); err != nil {
		return err
	}
	if err := setShortcutAppUserModelID(shortcutPath, appUserModelID); err != nil {
		return err
	}
	return nil
}

func writeShortcutLnkCOM(shortcutPath, targetExe, arguments, workingDir, description, iconLocation string) error {
	// CoInitialize: S_OK (0), S_FALSE (1) both require CoUninitialize; RPC_E_CHANGED_MODE means skip uninit.
	const rpcEChangedMode = uintptr(0x80010106)
	var needUninit bool
	if err := ole.CoInitialize(0); err != nil {
		oe, ok := err.(*ole.OleError)
		if !ok {
			return fmt.Errorf("com init: %w", err)
		}
		switch oe.Code() {
		case 1: // S_FALSE — COM already initialized on this thread; refcount balanced by Uninit
			needUninit = true
		case rpcEChangedMode:
			needUninit = false
		default:
			return fmt.Errorf("com init: %w", err)
		}
	} else {
		needUninit = true
	}
	if needUninit {
		defer ole.CoUninitialize()
	}

	unk, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return fmt.Errorf("create WScript.Shell: %w", err)
	}
	defer unk.Release()

	shell, err := unk.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("query IDispatch: %w", err)
	}
	defer shell.Release()

	v, err := oleutil.CallMethod(shell, "CreateShortcut", shortcutPath)
	if err != nil {
		return fmt.Errorf("CreateShortcut: %w", err)
	}
	defer v.Clear()

	link := v.ToIDispatch()
	if link == nil {
		return fmt.Errorf("CreateShortcut returned nil dispatch")
	}
	// IDispatch lifetime is owned by VARIANT; VariantClear releases it.

	if _, err := oleutil.PutProperty(link, "TargetPath", targetExe); err != nil {
		return fmt.Errorf("TargetPath: %w", err)
	}
	if _, err := oleutil.PutProperty(link, "Arguments", arguments); err != nil {
		return fmt.Errorf("Arguments: %w", err)
	}
	if strings.TrimSpace(workingDir) != "" {
		if _, err := oleutil.PutProperty(link, "WorkingDirectory", workingDir); err != nil {
			return fmt.Errorf("WorkingDirectory: %w", err)
		}
	}
	if strings.TrimSpace(description) != "" {
		if _, err := oleutil.PutProperty(link, "Description", description); err != nil {
			return fmt.Errorf("Description: %w", err)
		}
	}
	if strings.TrimSpace(iconLocation) != "" {
		if _, err := oleutil.PutProperty(link, "IconLocation", iconLocation); err != nil {
			return fmt.Errorf("IconLocation: %w", err)
		}
	}
	if _, err := oleutil.CallMethod(link, "Save"); err != nil {
		return fmt.Errorf("Save: %w", err)
	}
	return nil
}

// ReadLnkShortcut returns target exe, arguments, and icon location (e.g. "path,0") from a .lnk file.
func ReadLnkShortcut(lnkPath string) (target string, arguments string, iconLocation string, err error) {
	if lnkPath == "" {
		return "", "", "", fmt.Errorf("empty shortcut path")
	}
	b, err := os.ReadFile(lnkPath)
	if err != nil {
		return "", "", "", err
	}
	return parseLnk(b)
}
