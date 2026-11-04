//go:build !windows

package winutil

import "errors"

var errCredentialManagerWindowsOnly = errors.New("Windows Credential Manager is only available on Windows")

func ReadCredential(target string) (Credential, error) {
	return Credential{}, errCredentialManagerWindowsOnly
}

func WriteCredential(credential Credential) error {
	return errCredentialManagerWindowsOnly
}

func DeleteCredential(target string) error {
	return errCredentialManagerWindowsOnly
}

func DPAPIProtect(data []byte) ([]byte, error) {
	return nil, errCredentialManagerWindowsOnly
}

func DPAPIUnprotect(data []byte) ([]byte, error) {
	return nil, errCredentialManagerWindowsOnly
}
