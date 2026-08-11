//go:build windows

package steambrowser

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"TcNo-Acc-Switcher/internal/webview2"
	"TcNo-Acc-Switcher/internal/webview2/w32"
)

// Supported reports whether this build can open session windows.
func Supported() bool { return true }

// windowsView is the WebView2 content area, attached as a child of the Wails
// chrome window and isolated to one account by its profile.
//
// Every method here touches COM and must run on the thread that created the
// controller, which is the Wails main thread. Callers reach it through
// window.go's main-thread dispatch; nothing in this file dispatches for itself,
// so that the whole sequence a caller performs stays on one thread.
type windowsView struct {
	chromium   *webview2.Chromium
	platform   Platform
	hostWindow uintptr
	topInset   int

	// mu guards the cached state the event handlers write and the state
	// reporter reads. Handlers arrive on the UI thread, but the reporter is
	// invoked from them, so the lock keeps a partially updated snapshot from
	// escaping.
	mu      sync.Mutex
	url     string
	title   string
	loading bool
	canBack bool
	canFwd  bool

	onState     func(ViewState)
	onNewWindow func(string)

	closeOnce sync.Once
}

func newView(options ViewOptions) (View, error) {
	if options.NativeWindow == 0 {
		return nil, errors.New("steambrowser: no host window")
	}
	if err := webview2.ValidateProfileName(options.Profile); err != nil {
		return nil, err
	}

	view := &windowsView{
		platform:    options.Platform,
		hostWindow:  options.NativeWindow,
		onState:     options.OnState,
		onNewWindow: options.OnNewWindow,
	}

	chromium := webview2.NewChromium()
	chromium.DataPath = options.DataPath
	chromium.ProfileName = options.Profile
	// The content view shows a remote site, so it gets none of the capabilities
	// the application's own windows deny, and no developer tools in production.
	chromium.Debug = options.DevTools
	view.chromium = chromium

	if !chromium.Embed(options.NativeWindow) {
		return nil, errors.New("steambrowser: could not create the content view")
	}
	if env := chromium.Environment(); env == nil || !env.SupportsProfiles() {
		chromium.Close()
		return nil, webview2.ErrProfilesUnsupported
	}

	if err := view.plantCookies(options.Cookies); err != nil {
		chromium.Close()
		return nil, err
	}
	if err := view.subscribe(); err != nil {
		chromium.Close()
		return nil, err
	}

	view.topInset = options.ReservedTop
	if err := view.SetTopInset(options.ReservedTop); err != nil {
		chromium.Close()
		return nil, err
	}
	// Embed leaves the controller hidden; without this the window shows the
	// toolbar over an empty background and nothing else.
	if err := chromium.Show(); err != nil {
		chromium.Close()
		return nil, fmt.Errorf("steambrowser: show content view: %w", err)
	}

	if options.InitialURL != "" {
		if err := view.Navigate(options.InitialURL); err != nil {
			chromium.Close()
			return nil, err
		}
	}
	return view, nil
}

// plantCookies writes the account's session into this profile's jar. It runs
// before the first navigation, so the very first request is already signed in
// and Steam never sees an anonymous hit from this window.
func (v *windowsView) plantCookies(cookies []Cookie) error {
	if len(cookies) == 0 {
		return nil
	}
	jar, err := v.chromium.GetCookieManager()
	if err != nil {
		return fmt.Errorf("steambrowser: cookie manager: %w", err)
	}
	defer jar.Release()

	for _, c := range cookies {
		cookie, err := jar.CreateCookie(c.Name, c.Value, c.Domain, c.Path)
		if err != nil {
			return fmt.Errorf("steambrowser: build cookie %s for %s: %w", c.Name, c.Domain, err)
		}
		if err := jar.AddOrUpdateCookie(cookie); err != nil {
			return fmt.Errorf("steambrowser: write cookie %s for %s: %w", c.Name, c.Domain, err)
		}
	}
	return nil
}

func (v *windowsView) webview() (*webview2.ICoreWebView2_2, error) {
	view := v.chromium.GetWebView()
	if view == nil {
		return nil, errors.New("steambrowser: content view is gone")
	}
	view2, err := view.QueryInterface2()
	if err != nil {
		return nil, fmt.Errorf("steambrowser: ICoreWebView2_2: %w", err)
	}
	return view2, nil
}

// subscribe wires the events the toolbar is driven by. The handler objects are
// kept on the view because the runtime holds only raw pointers to them; letting
// Go collect one would leave the runtime calling into freed memory.
func (v *windowsView) subscribe() error {
	view2, err := v.webview()
	if err != nil {
		return err
	}
	if err := view2.AddSourceChanged(webview2.NewICoreWebView2SourceChangedEventHandler(v)); err != nil {
		return fmt.Errorf("steambrowser: subscribe to source changes: %w", err)
	}
	if err := view2.AddHistoryChanged(webview2.NewICoreWebView2HistoryChangedEventHandler(v)); err != nil {
		return fmt.Errorf("steambrowser: subscribe to history changes: %w", err)
	}
	if err := view2.AddDocumentTitleChanged(webview2.NewICoreWebView2DocumentTitleChangedEventHandler(v)); err != nil {
		return fmt.Errorf("steambrowser: subscribe to title changes: %w", err)
	}
	if err := view2.AddNavigationStarting(webview2.NewICoreWebView2NavigationStartingEventHandler(v)); err != nil {
		return fmt.Errorf("steambrowser: subscribe to navigation start: %w", err)
	}
	if err := view2.AddNavigationCompleted(webview2.NewICoreWebView2NavigationCompletedEventHandler(v)); err != nil {
		return fmt.Errorf("steambrowser: subscribe to navigation completion: %w", err)
	}
	if err := view2.AddNewWindowRequested(webview2.NewICoreWebView2NewWindowRequestedEventHandler(v)); err != nil {
		return fmt.Errorf("steambrowser: subscribe to new-window requests: %w", err)
	}
	return nil
}

func (v *windowsView) Navigate(url string) error {
	view2, err := v.webview()
	if err != nil {
		return err
	}
	return view2.Navigate(url)
}

func (v *windowsView) Reload() error {
	view2, err := v.webview()
	if err != nil {
		return err
	}
	return view2.Reload()
}

func (v *windowsView) Stop() error {
	view2, err := v.webview()
	if err != nil {
		return err
	}
	return view2.Stop()
}

func (v *windowsView) Back() error {
	view2, err := v.webview()
	if err != nil {
		return err
	}
	return view2.GoBack()
}

func (v *windowsView) Forward() error {
	view2, err := v.webview()
	if err != nil {
		return err
	}
	return view2.GoForward()
}

// SetTopInset fills the host window below top.
//
// The client rect is read from the window itself, so only the inset has to
// cross from the host's device-independent pixels into physical ones. Deriving
// the width and height here as well would mean converting them too, and a
// scaled display is exactly where that goes wrong.
func (v *windowsView) SetTopInset(top int) error {
	if v.chromium == nil {
		return errors.New("steambrowser: content view is gone")
	}
	if top < 0 {
		top = 0
	}
	v.topInset = top

	client, err := w32.GetClientRect(v.hostWindow)
	if err != nil {
		return fmt.Errorf("steambrowser: measure host window: %w", err)
	}
	if int32(top) > client.Bottom {
		top = int(client.Bottom)
	}
	v.chromium.ResizeWithBounds(&webview2.Rect{
		Left:   client.Left,
		Top:    client.Top + int32(top),
		Right:  client.Right,
		Bottom: client.Bottom,
	})
	return nil
}

func (v *windowsView) Close() {
	v.closeOnce.Do(func() {
		if v.chromium != nil {
			v.chromium.Close()
			v.chromium = nil
		}
	})
}

// --- COM callbacks ---
//
// The runtime calls these on the UI thread. They keep no reference to the
// arguments beyond the call.

func (v *windowsView) QueryInterface(_, _ uintptr) uintptr { return 0 }
func (v *windowsView) AddRef() uintptr                     { return 1 }
func (v *windowsView) Release() uintptr                    { return 1 }

func (v *windowsView) SourceChanged(*webview2.ICoreWebView2, uintptr) uintptr {
	v.refresh(nil)
	return 0
}

func (v *windowsView) HistoryChanged(*webview2.ICoreWebView2, uintptr) uintptr {
	v.refresh(nil)
	return 0
}

func (v *windowsView) DocumentTitleChanged(*webview2.ICoreWebView2, uintptr) uintptr {
	v.refresh(nil)
	return 0
}

// NavigationStarting refuses anything that is not http or https before it
// loads. A session window carries an account's cookies, so file:, javascript:
// and any registered protocol handler have no business running in one, whether
// they arrive from a link, a redirect or a script.
func (v *windowsView) NavigationStarting(_ *webview2.ICoreWebView2, raw *webview2.IUnknown) uintptr {
	if args := webview2.NavigationStartingArgs(raw); args != nil {
		url, err := args.GetUri()
		if err == nil && !navigableScheme(url) {
			_ = args.PutCancel(true)
			return 0
		}
	}
	loading := true
	v.refresh(&loading)
	return 0
}

// navigableScheme reports a scheme a session window may load. An empty or
// unparseable URL is refused rather than assumed harmless.
func navigableScheme(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
}

func (v *windowsView) NavigationCompleted(*webview2.ICoreWebView2, *webview2.ICoreWebView2NavigationCompletedEventArgs) uintptr {
	loading := false
	v.refresh(&loading)
	return 0
}

// NewWindowRequested suppresses the popup WebView2 would otherwise open and
// hands the URL back to the host, which decides between a session window and
// the user's own browser. Marking it handled is what stops a chromeless window
// carrying this account's session appearing outside the toolbar.
func (v *windowsView) NewWindowRequested(_ *webview2.ICoreWebView2, args *webview2.ICoreWebView2NewWindowRequestedEventArgs) uintptr {
	if args == nil {
		return 0
	}
	_ = args.PutHandled(true)
	url, err := args.GetUri()
	if err != nil || url == "" {
		return 0
	}
	if v.onNewWindow != nil {
		v.onNewWindow(url)
	}
	return 0
}

// refresh re-reads the page's current state and reports it. loading is passed
// only by the navigation events, which know it; the others leave it as it was.
func (v *windowsView) refresh(loading *bool) {
	view2, err := v.webview()
	if err != nil {
		return
	}
	url, _ := view2.GetSource()
	title, _ := view2.GetDocumentTitle()
	canBack, _ := view2.GetCanGoBack()
	canFwd, _ := view2.GetCanGoForward()

	v.mu.Lock()
	v.url, v.title, v.canBack, v.canFwd = url, title, canBack, canFwd
	if loading != nil {
		v.loading = *loading
	}
	state := stateFor(v.platform, v.url, v.title, v.loading, v.canBack, v.canFwd)
	report := v.onState
	v.mu.Unlock()

	if report != nil {
		report(state)
	}
}
