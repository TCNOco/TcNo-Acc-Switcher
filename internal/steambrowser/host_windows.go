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
	// contentWindow is the child window WebView2 created for this view. It is
	// tracked so the view can be kept in front of the host's own webview.
	contentWindow uintptr
	topInset      int

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
	onDownload  func(string)

	// handlers holds the event-handler objects for the view's lifetime.
	//
	// They are handed to WebView2 as raw pointers, which Go's garbage collector
	// cannot see. Without a reference here they become unreachable the moment
	// subscribe returns, and the next event the runtime raises calls into freed
	// memory - an access violation minutes later, once a collection has run.
	handlers []any

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
		onDownload:  options.OnDownload,
	}

	chromium := webview2.NewChromium()
	chromium.DataPath = options.DataPath
	chromium.ProfileName = options.Profile
	view.chromium = chromium

	// Snapshot the host's children so the one WebView2 adds can be told apart
	// from the host's own webview afterwards.
	before := childWindows(options.NativeWindow)
	if !chromium.Embed(options.NativeWindow) {
		chromium.Close()
		return nil, errors.New("steambrowser: could not create the content view")
	}
	view.contentWindow = newChildWindow(before, childWindows(options.NativeWindow))
	log := logger().With("profile", options.Profile)
	if view.contentWindow == 0 {
		log.Warn("content view window not identified; it cannot be kept in front of the host webview")
	} else {
		log.Debug("content view created",
			"hwnd", view.contentWindow, "class", windowClassName(view.contentWindow))
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
	if err := view.applySettings(options.DevTools); err != nil {
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
	view.raise()
	log.Info("content view ready",
		"hwnd", view.contentWindow,
		"visible", windowVisible(view.contentWindow),
		"topInset", options.ReservedTop)

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
		if err := cookie.PutIsSecure(c.Secure); err != nil {
			return fmt.Errorf("steambrowser: mark cookie %s secure: %w", c.Name, err)
		}
		if err := cookie.PutIsHttpOnly(c.HTTPOnly); err != nil {
			return fmt.Errorf("steambrowser: mark cookie %s http-only: %w", c.Name, err)
		}
		if err := cookie.PutSameSite(sameSiteKind(c.SameSite)); err != nil {
			return fmt.Errorf("steambrowser: set same-site on cookie %s: %w", c.Name, err)
		}
		if err := jar.AddOrUpdateCookie(cookie); err != nil {
			return fmt.Errorf("steambrowser: write cookie %s for %s: %w", c.Name, c.Domain, err)
		}
		cookie.Release()
	}
	return nil
}

// sameSiteKind maps to COREWEBVIEW2_COOKIE_SAME_SITE_KIND, whose values the IDL
// gives as NONE, LAX, STRICT in that order.
func sameSiteKind(policy SameSite) int32 {
	if policy == SameSiteNone {
		return 0
	}
	return 1
}

// webview returns the view's ICoreWebView2_2, and a function to release it.
//
// QueryInterface hands back a counted reference. Every call here happens on a
// navigation, a title change or a history change, so dropping the reference
// instead of releasing it leaks one per event on a busy page.
func (v *windowsView) webview() (*webview2.ICoreWebView2_2, func(), error) {
	if v.chromium == nil {
		return nil, nil, errors.New("steambrowser: content view is gone")
	}
	view := v.chromium.GetWebView()
	if view == nil {
		return nil, nil, errors.New("steambrowser: content view is gone")
	}
	view2, err := view.QueryInterface2()
	if err != nil {
		return nil, nil, fmt.Errorf("steambrowser: ICoreWebView2_2: %w", err)
	}
	return view2, func() { view2.Release() }, nil
}

// subscribe wires the events the toolbar is driven by. Every handler is kept in
// v.handlers, for the lifetime reason documented on that field.
func (v *windowsView) subscribe() error {
	view2, release, err := v.webview()
	if err != nil {
		return err
	}
	defer release()

	sourceChanged := webview2.NewICoreWebView2SourceChangedEventHandler(v)
	historyChanged := webview2.NewICoreWebView2HistoryChangedEventHandler(v)
	titleChanged := webview2.NewICoreWebView2DocumentTitleChangedEventHandler(v)
	navigationStarting := webview2.NewICoreWebView2NavigationStartingEventHandler(v)
	navigationCompleted := webview2.NewICoreWebView2NavigationCompletedEventHandler(v)
	newWindowRequested := webview2.NewICoreWebView2NewWindowRequestedEventHandler(v)
	v.handlers = append(v.handlers,
		sourceChanged, historyChanged, titleChanged,
		navigationStarting, navigationCompleted, newWindowRequested,
	)

	if err := view2.AddSourceChanged(sourceChanged); err != nil {
		return fmt.Errorf("steambrowser: subscribe to source changes: %w", err)
	}
	if err := view2.AddHistoryChanged(historyChanged); err != nil {
		return fmt.Errorf("steambrowser: subscribe to history changes: %w", err)
	}
	if err := view2.AddDocumentTitleChanged(titleChanged); err != nil {
		return fmt.Errorf("steambrowser: subscribe to title changes: %w", err)
	}
	if err := view2.AddNavigationStarting(navigationStarting); err != nil {
		return fmt.Errorf("steambrowser: subscribe to navigation start: %w", err)
	}
	if err := view2.AddNavigationCompleted(navigationCompleted); err != nil {
		return fmt.Errorf("steambrowser: subscribe to navigation completion: %w", err)
	}
	if err := view2.AddNewWindowRequested(newWindowRequested); err != nil {
		return fmt.Errorf("steambrowser: subscribe to new-window requests: %w", err)
	}
	return v.subscribeDownloads()
}

// subscribeDownloads wires the download event, which lives on a later interface
// than everything else this view uses.
//
// A runtime too old to offer it is not fatal: the window works, and downloads
// land in WebView2's own flyout instead of the user's browser.
func (v *windowsView) subscribeDownloads() error {
	if v.chromium == nil {
		return errors.New("steambrowser: content view is gone")
	}
	view := v.chromium.GetWebView()
	if view == nil {
		return errors.New("steambrowser: content view is gone")
	}
	view4, err := view.QueryInterface4()
	if err != nil {
		logger().Warn("downloads cannot be handed to the browser: this WebView2 runtime is too old", "error", err)
		return nil
	}
	defer view4.Release()

	downloadStarting := webview2.NewICoreWebView2DownloadStartingEventHandler(v)
	v.handlers = append(v.handlers, downloadStarting)
	if err := view4.AddDownloadStarting(downloadStarting); err != nil {
		return fmt.Errorf("steambrowser: subscribe to downloads: %w", err)
	}
	return nil
}

func (v *windowsView) Navigate(url string) error {
	view2, release, err := v.webview()
	if err != nil {
		return err
	}
	defer release()
	return view2.Navigate(url)
}

func (v *windowsView) Reload() error {
	view2, release, err := v.webview()
	if err != nil {
		return err
	}
	defer release()
	return view2.Reload()
}

func (v *windowsView) Stop() error {
	view2, release, err := v.webview()
	if err != nil {
		return err
	}
	defer release()
	return view2.Stop()
}

func (v *windowsView) Back() error {
	view2, release, err := v.webview()
	if err != nil {
		return err
	}
	defer release()
	return view2.GoBack()
}

func (v *windowsView) Forward() error {
	view2, release, err := v.webview()
	if err != nil {
		return err
	}
	defer release()
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

	// The host window is frameless, so its resize handles are a strip inside the
	// client area rather than a non-client border. A child window covering the
	// client area to its edges takes the mouse first and the window stops being
	// resizable, so the sides and bottom stop short by that strip. The top does
	// not need it: the toolbar is already above.
	//
	// A maximised window has no resize handles, so it gets the full width.
	edge := int32(0)
	if !windowMaximised(v.hostWindow) {
		edge = int32(resizeBorder())
	}

	// PutBounds rather than Chromium.ResizeWithBounds: that path logs a failure
	// and carries on, so a placement that fails reaches no caller.
	controller := v.chromium.GetController()
	if controller == nil {
		return errors.New("steambrowser: content view has no controller")
	}
	bounds := w32.Rect{
		Left:   client.Left + edge,
		Top:    client.Top + int32(top),
		Right:  client.Right - edge,
		Bottom: client.Bottom - edge,
	}
	if bounds.Right < bounds.Left {
		bounds.Right = bounds.Left
	}
	if bounds.Bottom < bounds.Top {
		bounds.Bottom = bounds.Top
	}
	if err := controller.PutBounds(bounds); err != nil {
		return fmt.Errorf("steambrowser: place content view: %w", err)
	}
	// The host re-lays its own webview out on resize, so the content view's
	// position in the z-order is re-asserted every time it is placed.
	v.raise()
	logger().Debug("content view placed",
		"hwnd", v.contentWindow, "top", bounds.Top, "resizeEdge", edge,
		"width", bounds.Right-bounds.Left, "height", bounds.Bottom-bounds.Top,
		"visible", windowVisible(v.contentWindow))
	return nil
}

// raise keeps the content view in front of the host window's own webview. They
// are siblings, and a content view that slips behind is invisible while still
// reporting a loaded page - which reads as a page that never loads.
func (v *windowsView) raise() {
	if v.contentWindow == 0 {
		return
	}
	if err := raiseWindow(v.contentWindow); err != nil {
		logger().Warn("could not raise the content view", "error", err)
	}
}

func (v *windowsView) Close() {
	v.closeOnce.Do(func() {
		if v.chromium != nil {
			v.chromium.Close()
			v.chromium = nil
		}
		// Only now that nothing can raise an event into them.
		v.handlers = nil
	})
}

// COM callbacks. The runtime calls these on the UI thread, and they keep no
// reference to the arguments beyond the call.

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

// DownloadStarting refuses the transfer and hands the address to the host, which
// gives it to the user's own browser.
//
// The address is read before anything is cancelled: a download this window
// cannot pass on - a blob or a data URL, which mean nothing outside the page
// that made them - is left to proceed rather than dropped with nowhere to go.
func (v *windowsView) DownloadStarting(_ *webview2.ICoreWebView2, args *webview2.ICoreWebView2DownloadStartingEventArgs) uintptr {
	if args == nil {
		return 0
	}
	operation, err := args.GetDownloadOperation()
	if err != nil || operation == nil {
		logger().Warn("a download could not be read and was left to the content view", "error", err)
		return 0
	}
	defer operation.Release()

	url, err := operation.GetUri()
	if err != nil || !navigableScheme(url) {
		return 0
	}
	// Handled as well as cancelled: cancelling alone still raises the flyout,
	// which would announce a download that is not happening here.
	if err := args.PutHandled(true); err != nil {
		logger().Warn("could not suppress the download flyout", "error", err)
	}
	if err := args.PutCancel(true); err != nil {
		// Refusing to cancel means the file is coming down here anyway, so it must
		// not also be handed to the browser and fetched twice.
		logger().Warn("a download could not be cancelled and stays in this window", "error", err)
		return 0
	}
	if v.onDownload != nil {
		v.onDownload(url)
	}
	return 0
}

// refresh re-reads the page's current state and reports it. loading is passed
// only by the navigation events, which know it; the others leave it as it was.
func (v *windowsView) refresh(loading *bool) {
	view2, release, err := v.webview()
	if err != nil {
		return
	}
	defer release()
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

// applySettings decides what the remote page is allowed to do.
//
// Developer tools follow the build: on in a debug build, where they are the
// only way to see why a page misbehaves, and off in a release one, where they
// would hand a page's console to anyone who reaches this window.
//
// The default context menu stays on, unlike everywhere else in the application.
// It is what supplies "open link in new window", which is how a link reaches
// another session window rather than a chromeless popup.
func (v *windowsView) applySettings(devTools bool) error {
	view2, release, err := v.webview()
	if err != nil {
		return err
	}
	defer release()

	settings, err := view2.GetSettings()
	if err != nil {
		return fmt.Errorf("steambrowser: read view settings: %w", err)
	}
	if err := settings.PutAreDevToolsEnabled(devTools); err != nil {
		return fmt.Errorf("steambrowser: set developer tools: %w", err)
	}
	if err := settings.PutAreDefaultContextMenusEnabled(true); err != nil {
		return fmt.Errorf("steambrowser: set context menu: %w", err)
	}
	return nil
}

// OpenDevTools opens the content view's developer tools, for working out why a
// page behaves differently here than in a browser.
func (v *windowsView) OpenDevTools() error {
	if v.chromium == nil {
		return errors.New("steambrowser: content view is gone")
	}
	v.chromium.OpenDevToolsWindow()
	return nil
}
