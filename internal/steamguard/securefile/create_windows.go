//go:build windows

package securefile

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ErrHardeningUnsupported = errors.New("owner-only file protection is unavailable")

func CreateNew(path string) (*os.File, error) {
	attributes, err := protectedSecurityAttributes()
	if err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,
		attributes,
		windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_WRITE_THROUGH,
		0,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("create protected file handle")
	}
	if err := verifyProtected(path); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return file, nil
}

// CreateDirectoryNew creates a directory with an owner-and-SYSTEM-only,
// inheritance-protected DACL before the directory becomes visible.
func CreateDirectoryNew(path string) error {
	attributes, err := protectedSecurityAttributes()
	if err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if err := windows.CreateDirectory(name, attributes); err != nil {
		return err
	}
	if err := verifyProtected(path); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func protectedSecurityAttributes() (*windows.SecurityAttributes, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return nil, errors.Join(ErrHardeningUnsupported, err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return nil, errors.Join(ErrHardeningUnsupported, err)
	}
	sddl := "D:P(A;;FA;;;SY)(A;;FA;;;" + user.User.Sid.String() + ")"
	descriptor, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, errors.Join(ErrHardeningUnsupported, err)
	}
	attributes := windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: descriptor,
	}
	return &attributes, nil
}

func verifyProtected(path string) error {
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return errors.Join(ErrHardeningUnsupported, err)
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return errors.Join(ErrHardeningUnsupported, err)
	}
	return nil
}
