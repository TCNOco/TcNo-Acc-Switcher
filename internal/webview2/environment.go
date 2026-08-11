//go:build windows

package webview2

import (
	"errors"
	"fmt"
)

const maxProfileNameLen = 64

// ErrInvalidProfileName rejects a name WebView2 would not accept as a directory.
var ErrInvalidProfileName = errors.New("webview2: invalid profile name")

// ValidateProfileName accepts the conservative subset of what WebView2 allows:
// ASCII letters, digits, dot, dash and underscore. WebView2 permits more, but a
// profile name becomes a directory name, so the narrow set is the one worth
// relying on. It must also not lead with a dot, which would hide the directory
// and, on some shells, read as a relative path.
func ValidateProfileName(name string) error {
	if name == "" || len(name) > maxProfileNameLen {
		return fmt.Errorf("%w: length %d", ErrInvalidProfileName, len(name))
	}
	if name[0] == '.' || name[0] == '-' {
		return fmt.Errorf("%w: %q must not start with %q", ErrInvalidProfileName, name, name[0:1])
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.' || c == '-' || c == '_':
		default:
			return fmt.Errorf("%w: %q contains %q", ErrInvalidProfileName, name, string(c))
		}
	}
	return nil
}

// Environments are not created directly here, deliberately.
//
// Calling the loader's CreateCoreWebView2EnvironmentWithOptions from application
// code yields an environment that answers QueryInterface but fails every real call
// with RPC_E_WRONG_THREAD, as though it belonged to another apartment — including
// when the call is made on the very thread this package's init pinned, and whether
// the completion is awaited with a blocking or a polling message loop. Going
// through Chromium.Embed, which is the path Wails itself exercises, produces an
// environment that works. The difference was reproduced by A/B in one process, so
// Embed is the supported route and the only one used here.
//
// Each browser window therefore gets its own Chromium, distinguished by
// Chromium.ProfileName and sharing one DataPath. That is Chromium's intended
// granularity, and it keeps this package on proven ground.
