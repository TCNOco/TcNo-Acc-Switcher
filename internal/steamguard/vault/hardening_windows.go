//go:build windows

package vault

import (
	"fmt"

	"golang.org/x/sys/windows"
)

type platformHardener struct{}

func defaultHardener() Hardener { return platformHardener{} }

func (platformHardener) HardenDir(path string) error  { return hardenWindowsPath(path) }
func (platformHardener) HardenFile(path string) error { return hardenWindowsPath(path) }

func hardenWindowsPath(path string) error {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return fmt.Errorf("%w: open process token: %v", ErrHardeningUnsupported, err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("%w: get token user: %v", ErrHardeningUnsupported, err)
	}
	sddl := "D:P(A;;FA;;;SY)(A;;FA;;;" + user.User.Sid.String() + ")"
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return fmt.Errorf("%w: build DACL: %v", ErrHardeningUnsupported, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("%w: read DACL: %v", ErrHardeningUnsupported, err)
	}
	err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil)
	if err != nil {
		return fmt.Errorf("apply owner-only DACL to %q: %w", path, err)
	}
	return nil
}
