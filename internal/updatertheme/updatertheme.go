package updatertheme

import (
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

const defaultCSS = `:root {
  --bg: #28293a;
  --surface: #333333;
  --surface-2: #444444;
  --fg: #eceff8;
  --fg-dim: #959bb0;
  --fg-faint: #676d87;
  --border: #6272a4;
  --accent: #ffaa00;
  --accent-fg: #1d1d1f;
  --success: #7cbd6e;
  --error: #e85d5d;
  --radius: 10px;
}`

var (
	mu     sync.RWMutex
	css    = defaultCSS
	window *updater.BuiltinWindow
)

// SetWindow registers the BuiltinWindow passed to app.Updater.Init so CSS can
// be refreshed when the user changes theme without re-initialising the updater.
func SetWindow(w *updater.BuiltinWindow) {
	mu.Lock()
	window = w
	mu.Unlock()
}

// CurrentCSS returns the updater stylesheet injected into the built-in window.
func CurrentCSS() string {
	mu.RLock()
	defer mu.RUnlock()
	if css == "" {
		return defaultCSS
	}
	return css
}

// SetCSS updates the updater window theme. The frontend builds this from the
// active app theme's computed CSS variables.
func SetCSS(next string) {
	next = strings.TrimSpace(next)
	if next == "" {
		next = defaultCSS
	}
	mu.Lock()
	css = next
	if window != nil {
		window.CSS = next
	}
	mu.Unlock()
}

// NewBuiltinWindow creates the Wails updater window config for Init.
func NewBuiltinWindow() *updater.BuiltinWindow {
	return &updater.BuiltinWindow{
		CSS: CurrentCSS(),
		Options: updater.WindowOptions{
			Title: "TcNo Account Switcher Update",
		},
	}
}
