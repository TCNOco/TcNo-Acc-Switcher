//go:build windows

package winutil

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const protocolScheme = "tcno"

// IsProtocolRegistered reports whether HKCR\tcno has URL Protocol set (custom URL scheme).
func IsProtocolRegistered() bool {
	k, err := registry.OpenKey(registry.CLASSES_ROOT, protocolScheme, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue("URL Protocol")
	return err == nil && strings.TrimSpace(v) != ""
}

// RegisterProtocol writes HKCR\tcno with Shell\Open\Command pointing at exe with "%1".
func RegisterProtocol(exePath string) error {
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return fmt.Errorf("empty exe path")
	}

	k, _, err := registry.CreateKey(registry.CLASSES_ROOT, protocolScheme, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if err := k.SetStringValue("", "URL:TcNo Account Switcher"); err != nil {
		return err
	}
	if err := k.SetStringValue("URL Protocol", ""); err != nil {
		return err
	}

	sub := protocolScheme + `\shell\open\command`
	cmdKey, _, err := registry.CreateKey(registry.CLASSES_ROOT, sub, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer cmdKey.Close()

	cmd := fmt.Sprintf("\"%s\" \"%%1\"", exePath)
	return cmdKey.SetStringValue("", cmd)
}

// UnregisterProtocol removes HKCR\tcno (best-effort; delete children first).
func UnregisterProtocol() error {
	_ = registry.DeleteKey(registry.CLASSES_ROOT, protocolScheme+`\shell\open\command`)
	_ = registry.DeleteKey(registry.CLASSES_ROOT, protocolScheme+`\shell\open`)
	_ = registry.DeleteKey(registry.CLASSES_ROOT, protocolScheme+`\shell`)
	return registry.DeleteKey(registry.CLASSES_ROOT, protocolScheme)
}
