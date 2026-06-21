//go:build windows

package winutil

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

const gpsReadWrite = 0x2

var (
	modPropsys                            = windows.NewLazySystemDLL("propsys.dll")
	modShell32                            = windows.NewLazySystemDLL("shell32.dll")
	procInitPropVariantFromString         = modPropsys.NewProc("InitPropVariantFromString")
	procPropVariantClear                  = modPropsys.NewProc("PropVariantClear")
	procSHGetPropertyStoreFromParsingName = modShell32.NewProc("SHGetPropertyStoreFromParsingName")
)

type propertyKey struct {
	fmtid windows.GUID
	pid   uint32
}

var pkeyAppUserModelID = propertyKey{
	fmtid: windows.GUID{
		Data1: 0x9f4c2855, Data2: 0x9f79, Data3: 0x4f39,
		Data4: [8]byte{0xa8, 0xd0, 0xe1, 0xd4, 0x2d, 0xe1, 0xd5, 0xf3},
	},
	pid: 5,
}

type propVariant struct {
	data [propVariantSize]byte
}

type iPropertyStoreVtbl struct {
	queryInterface uintptr
	addRef         uintptr
	release        uintptr
	getCount       uintptr
	getAt          uintptr
	getValue       uintptr
	setValue       uintptr
	commit         uintptr
}

type iPropertyStore struct {
	vtbl *iPropertyStoreVtbl
}

func (ps *iPropertyStore) release() {
	syscall.SyscallN(ps.vtbl.release, uintptr(unsafe.Pointer(ps)))
}

func (ps *iPropertyStore) setValue(key *propertyKey, val *propVariant) error {
	hr, _, _ := syscall.SyscallN(
		ps.vtbl.setValue,
		uintptr(unsafe.Pointer(ps)),
		uintptr(unsafe.Pointer(key)),
		uintptr(unsafe.Pointer(val)),
	)
	return hresultErr(hr)
}

func (ps *iPropertyStore) commit() error {
	hr, _, _ := syscall.SyscallN(ps.vtbl.commit, uintptr(unsafe.Pointer(ps)))
	return hresultErr(hr)
}

func hresultErr(hr uintptr) error {
	if hr == 0 {
		return nil
	}
	return ole.NewError(hr)
}

func initPropVariantFromString(s string) (propVariant, error) {
	var pv propVariant
	ws, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return pv, err
	}
	hr, _, _ := procInitPropVariantFromString.Call(
		uintptr(unsafe.Pointer(ws)),
		uintptr(unsafe.Pointer(&pv)),
	)
	if e := hresultErr(hr); e != nil {
		return pv, e
	}
	return pv, nil
}

func clearPropVariant(pv *propVariant) {
	if pv == nil {
		return
	}
	procPropVariantClear.Call(uintptr(unsafe.Pointer(pv)))
}

func setShortcutAppUserModelID(lnkPath, appID string) error {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil
	}
	if len(appID) > 128 {
		appID = appID[:128]
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const rpcEChangedMode = uintptr(0x80010106)
	var needUninit bool
	if err := ole.CoInitialize(0); err != nil {
		oe, ok := err.(*ole.OleError)
		if !ok {
			return fmt.Errorf("com init: %w", err)
		}
		switch oe.Code() {
		case 1:
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

	pathPtr, err := windows.UTF16PtrFromString(lnkPath)
	if err != nil {
		return err
	}

	iid := ole.NewGUID("{886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99}")
	var punk uintptr
	hr, _, _ := procSHGetPropertyStoreFromParsingName.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		gpsReadWrite,
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&punk)),
	)
	if e := hresultErr(hr); e != nil {
		return fmt.Errorf("SHGetPropertyStoreFromParsingName: %w", e)
	}
	if punk == 0 {
		return fmt.Errorf("SHGetPropertyStoreFromParsingName: nil store")
	}
	ps := (*iPropertyStore)(unsafe.Pointer(punk))
	defer ps.release()

	pv, err := initPropVariantFromString(appID)
	if err != nil {
		return err
	}
	defer clearPropVariant(&pv)

	if err := ps.setValue(&pkeyAppUserModelID, &pv); err != nil {
		return fmt.Errorf("SetValue AppUserModelID: %w", err)
	}
	if err := ps.commit(); err != nil {
		return fmt.Errorf("Commit: %w", err)
	}
	return nil
}
