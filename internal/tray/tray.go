package tray

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync/atomic"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"

	"github.com/nfnt/resize"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	_ "image/gif"
	_ "image/jpeg"
	_ "golang.org/x/image/webp"
)

// Deps wires the tray to swap backends without import cycles.
type Deps struct {
	SwapBasic func(platformKey, uniqueID string) error
	SwapSteam func(steamID64 string, personaState int) error
}

// Manager owns the Wails system tray and menu rebuilds.
type Manager struct {
	app     *application.App
	win     *application.WebviewWindow
	systray *application.SystemTray
	deps    Deps

	quitting atomic.Bool
}

// NewManager constructs a tray manager (does not Run the tray yet).
func NewManager(app *application.App, win *application.WebviewWindow, deps Deps) *Manager {
	return &Manager{app: app, win: win, deps: deps}
}

// SetQuitting marks the next close as a real shutdown (do not minimize to tray).
func (m *Manager) SetQuitting(v bool) {
	m.quitting.Store(v)
}

// IsQuitting reports whether a full application quit was requested.
func (m *Manager) IsQuitting() bool {
	return m.quitting.Load()
}

// RegisterCloseHook intercepts window close to optionally hide to tray instead of destroying the window.
func (m *Manager) RegisterCloseHook() {
	if m.win == nil {
		return
	}
	m.win.RegisterHook(events.Common.WindowClosing, func(ev *application.WindowEvent) {
		if m.IsQuitting() {
			return
		}
		exeDir, err := platform.ResolveExeDir()
		if err != nil {
			return
		}
		s, err := platform.LoadAppSettings(exeDir)
		if err != nil || !s.ExitToTray {
			return
		}
		ev.Cancel()
		m.win.Hide()
	})
}

// MaybeHideMainWindow hides the main window when MinimizeOnSwitch is enabled in app settings.
func MaybeHideMainWindow() {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return
	}
	s, err := platform.LoadAppSettings(exeDir)
	if err != nil || !s.MinimizeOnSwitch {
		return
	}
	application.InvokeSync(func() {
		app := application.Get()
		if app == nil {
			return
		}
		w := app.Window.Current()
		if w == nil {
			return
		}
		w.Hide()
	})
}

// Start creates the systray, wires menu actions, and calls Run().
func (m *Manager) Start(iconPNG []byte) {
	if m.app == nil {
		return
	}
	m.systray = m.app.SystemTray.New()
	if len(iconPNG) > 0 {
		m.systray.SetIcon(iconPNG)
	}
	m.systray.SetLabel("TcNo Account Switcher")
	m.rebuildMenuLocked()

	m.systray.OnDoubleClick(func() {
		m.showHome()
	})

	m.app.OnShutdown(func() {
		if m.systray != nil {
			m.systray.Destroy()
		}
	})

	m.systray.Run()
}

// RefreshMenu rebuilds the tray menu from disk (call after MRU updates).
func (m *Manager) RefreshMenu() {
	if m.systray == nil {
		return
	}
	application.InvokeSync(m.rebuildMenuLocked)
}

func (m *Manager) rebuildMenuLocked() {
	if m.systray == nil {
		return
	}
	menu := application.NewMenu()

	menu.Add("TcNo Account Switcher").OnClick(func(_ *application.Context) {
		m.showHome()
	})

	users, err := LoadUsers()
	if err != nil {
		slog.Default().Warn("tray: load users", slog.Any("err", err))
		users = map[string][]TrayUser{}
	}
	keys := make([]string, 0, len(users))
	for k := range users {
		keys = append(keys, k)
	}
	// stable submenu order
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if strings.ToLower(keys[i]) > strings.ToLower(keys[j]) {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	for _, plat := range keys {
		list := users[plat]
		if len(list) == 0 {
			continue
		}
		sub := menu.AddSubmenu(plat)
		for _, u := range list {
			u := u
			plat := plat
			item := sub.Add("Switch to: " + u.Name)
			if b := menuBitmapForAccount(plat, u.Arg); len(b) > 0 {
				item.SetBitmap(b)
			}
			item.OnClick(func(_ *application.Context) {
				if err := m.handleAccountClick(plat, u.Arg); err != nil {
					slog.Default().Warn("tray switch failed", slog.Any("err", err))
				}
				m.RefreshMenu()
			})
		}
	}

	menu.AddSeparator()
	menu.Add("Exit").OnClick(func(_ *application.Context) {
		m.SetQuitting(true)
		m.app.Quit()
	})

	m.systray.SetMenu(menu)
}

func (m *Manager) showHome() {
	if m.win != nil {
		m.win.Show().Focus()
	}
	if m.app != nil {
		_ = m.app.Event.Emit("navigate", `{"page":"home"}`)
	}
}

func (m *Manager) handleAccountClick(platformKey, arg string) error {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return fmt.Errorf("empty tray arg")
	}
	if strings.HasPrefix(strings.ToLower(arg), "+s:") {
		id := strings.TrimSpace(arg[3:])
		if m.deps.SwapSteam == nil {
			return fmt.Errorf("steam swap not wired")
		}
		return m.deps.SwapSteam(id, -1)
	}
	if strings.HasPrefix(arg, "+") {
		rest := strings.TrimPrefix(arg, "+")
		i := strings.Index(rest, ":")
		if i <= 0 || i >= len(rest)-1 {
			return fmt.Errorf("bad tray arg")
		}
		uid := strings.TrimSpace(rest[i+1:])
		if m.deps.SwapBasic == nil {
			return fmt.Errorf("basic swap not wired")
		}
		return m.deps.SwapBasic(platformKey, uid)
	}
	return fmt.Errorf("unrecognized tray arg")
}

func menuBitmapForAccount(platformKey, arg string) []byte {
	if runtime.GOOS != "windows" {
		return nil
	}
	uid := trayUniqueFromArg(arg)
	if uid == "" {
		return nil
	}
	p, ok := profileimage.CachedFilePath(platformKey, uid)
	if !ok {
		return nil
	}
	b, err := os.ReadFile(p)
	if err != nil || len(b) == 0 {
		return nil
	}
	if len(b) > 512*1024 {
		return nil
	}
	// Wails validates menu bitmaps as PNG; cache may be WebP/JPEG/GIF.
	return cachedImageBytesAsMenuPNG(b)
}

func cachedImageBytesAsMenuPNG(b []byte) []byte {
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	img = scaleDownForTrayMenu(img, 32)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	out := buf.Bytes()
	if len(out) > 512*1024 {
		return nil
	}
	return out
}

func scaleDownForTrayMenu(img image.Image, maxSide int) image.Image {
	if maxSide <= 0 || img == nil {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxSide && h <= maxSide {
		return img
	}
	var nw, nh uint
	if w >= h {
		nw = uint(maxSide)
		nh = uint(int64(h) * int64(maxSide) / int64(w))
		if nh < 1 {
			nh = 1
		}
	} else {
		nh = uint(maxSide)
		nw = uint(int64(w) * int64(maxSide) / int64(h))
		if nw < 1 {
			nw = 1
		}
	}
	return resize.Resize(nw, nh, img, resize.Lanczos2)
}

func trayUniqueFromArg(arg string) string {
	arg = strings.TrimSpace(arg)
	if strings.HasPrefix(strings.ToLower(arg), "+s:") {
		return strings.TrimSpace(arg[3:])
	}
	if strings.HasPrefix(arg, "+") {
		rest := strings.TrimPrefix(arg, "+")
		i := strings.Index(rest, ":")
		if i >= 0 && i < len(rest)-1 {
			return strings.TrimSpace(rest[i+1:])
		}
	}
	return ""
}
