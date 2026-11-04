//go:build windows

package winutil

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"unsafe"

	"github.com/danieljoos/wincred"
	"golang.org/x/sys/windows"
)

const maxGenericCredentialBlob = 2560

// ReadCredential reads a generic credential from the current user's Windows
// Credential Manager. It supports the LegacyGeneric target spelling used by
// the Epic Games Launcher and never enumerates unrelated credentials.
func ReadCredential(target string) (Credential, error) {
	for _, candidate := range CredentialTargetCandidates(target) {
		item, err := wincred.GetGenericCredential(candidate)
		if err != nil {
			if errors.Is(err, wincred.ErrElementNotFound) {
				continue
			}
			return Credential{}, fmt.Errorf("read generic credential: %w", err)
		}

		blob := append([]byte(nil), item.CredentialBlob...)
		clear(item.CredentialBlob)
		return Credential{
			TargetName:  item.TargetName,
			UserName:    item.UserName,
			TargetAlias: item.TargetAlias,
			Comment:     item.Comment,
			Persist:     uint32(item.Persist),
			Blob:        blob,
		}, nil
	}
	return Credential{}, ErrCredentialNotFound
}

// WriteCredential updates only the exact target recorded in the snapshot.
// Credential blobs are limited by Windows to 2560 bytes.
func WriteCredential(credential Credential) error {
	target := strings.TrimSpace(credential.TargetName)
	if target == "" {
		return errors.New("credential target is empty")
	}
	if len(credential.Blob) == 0 {
		return errors.New("credential blob is empty")
	}
	if len(credential.Blob) > maxGenericCredentialBlob {
		return fmt.Errorf("credential blob exceeds the Windows limit of %d bytes", maxGenericCredentialBlob)
	}
	if credential.Persist < uint32(wincred.PersistSession) || credential.Persist > uint32(wincred.PersistEnterprise) {
		return fmt.Errorf("unsupported credential persistence value %d", credential.Persist)
	}

	item := wincred.NewGenericCredential(target)
	item.UserName = credential.UserName
	item.TargetAlias = credential.TargetAlias
	item.Comment = credential.Comment
	item.Persist = wincred.CredentialPersistence(credential.Persist)
	item.CredentialBlob = append([]byte(nil), credential.Blob...)
	defer clear(item.CredentialBlob)
	if err := item.Write(); err != nil {
		return fmt.Errorf("write generic credential: %w", err)
	}
	return nil
}

// DeleteCredential removes the requested target and its LegacyGeneric alias
// where present. It does not enumerate or touch any other stored credentials.
func DeleteCredential(target string) error {
	found := false
	for _, candidate := range CredentialTargetCandidates(target) {
		item, err := wincred.GetGenericCredential(candidate)
		if err != nil {
			if errors.Is(err, wincred.ErrElementNotFound) {
				continue
			}
			return fmt.Errorf("read credential before delete: %w", err)
		}
		deleteErr := item.Delete()
		clear(item.CredentialBlob)
		if deleteErr != nil {
			if errors.Is(deleteErr, wincred.ErrElementNotFound) {
				continue
			}
			return fmt.Errorf("delete generic credential: %w", deleteErr)
		}
		found = true
	}
	if !found {
		return ErrCredentialNotFound
	}
	return nil
}

// DPAPIProtect encrypts data for the current Windows user. Saved launcher
// snapshots are therefore not portable to another Windows user or computer.
func DPAPIProtect(data []byte) ([]byte, error) {
	return transformDPAPI(data, true)
}

// DPAPIUnprotect decrypts data protected for the current Windows user.
func DPAPIUnprotect(data []byte) ([]byte, error) {
	return transformDPAPI(data, false)
}

func transformDPAPI(data []byte, protect bool) ([]byte, error) {
	if uint64(len(data)) > uint64(^uint32(0)) {
		return nil, errors.New("data is too large for DPAPI")
	}

	input := windows.DataBlob{Size: uint32(len(data))}
	if len(data) > 0 {
		input.Data = &data[0]
	}
	var output windows.DataBlob

	var err error
	if protect {
		err = windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	} else {
		err = windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	}
	runtime.KeepAlive(data)
	if err != nil {
		action := "CryptUnprotectData"
		if protect {
			action = "CryptProtectData"
		}
		return nil, fmt.Errorf("%s: %w", action, err)
	}
	if output.Size > 0 && output.Data == nil {
		return nil, errors.New("DPAPI returned an empty output buffer")
	}

	var result []byte
	if output.Size > 0 {
		result = append([]byte(nil), unsafe.Slice(output.Data, int(output.Size))...)
	}
	// Clear decrypted native memory before releasing it.
	if output.Data != nil {
		if !protect && output.Size > 0 {
			clear(unsafe.Slice(output.Data, int(output.Size)))
		}
		_, _ = windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(output.Data))))
	}
	return result, nil
}
