package steambrowser

import "errors"

// ErrUnsupportedPlatform reports a build with no content-view backend. Only
// Windows has one today; see the notes on View for what a macOS or Linux
// implementation has to provide.
var ErrUnsupportedPlatform = errors.New("steambrowser: session windows are not supported on this platform")

// ViewState is everything the toolbar draws itself from. It is pushed to the
// frontend on every change rather than polled, so the URL bar and the
// back/forward buttons cannot drift from the page.
type ViewState struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Loading bool   `json:"loading"`
	CanBack bool   `json:"canGoBack"`
	CanFwd  bool   `json:"canGoForward"`
	// Trusted mirrors Classify for URL, computed here rather than in the
	// frontend so a value crossing the boundary cannot widen the trust list.
	Trusted bool   `json:"trusted"`
	Secure  bool   `json:"secure"`
	Host    string `json:"host"`
}

// ViewOptions describes a content view at creation.
type ViewOptions struct {
	// NativeWindow is the host window the view is attached to: an HWND on
	// Windows, an NSWindow on macOS, a GtkWindow on Linux. It comes from the
	// Wails window's NativeWindow().
	NativeWindow uintptr
	// Profile isolates this view's cookies and storage from every other view.
	Profile string
	// DataPath is the user data folder every profile lives under.
	DataPath string
	// DevTools enables the content view's developer tools. Off in production.
	DevTools bool
	// ReservedTop is how far down the host window's client area the view
	// starts, in physical pixels. It covers everything drawn above the page:
	// the application's title bar as well as the toolbar.
	ReservedTop int
	// Cookies are planted before the first navigation, which is what makes the
	// window browse as the account.
	Cookies []Cookie
	// InitialURL is loaded once the cookies are in place.
	InitialURL string
	// Platform selects the trust list applied to navigation.
	Platform Platform

	// OnState is called whenever the page, title or history changes.
	OnState func(ViewState)
	// OnNewWindow is called for a middle-click, a ctrl-click or a popup. The
	// view never opens one itself; returning tells the host what happened, and
	// the request is always suppressed in the content view.
	OnNewWindow func(url string)
	// OnDownload is called instead of downloading. A session window is for
	// browsing as an account, not for collecting files: it has no download UI,
	// no downloads folder of its own, and nothing to show a failed transfer in.
	// The transfer is cancelled before it starts and handed on here.
	OnDownload func(url string)
}

// View is one account's content area, the part of the window showing the site.
//
// Implementations are per-platform and every method is expected to be called
// from the platform's UI thread. The set is deliberately the intersection of
// what WebView2, WKWebView and WebKitGTK all offer natively, so no backend has
// to emulate anything:
//
//	Navigate   WebView2 Navigate      / WKWebView load        / webkit_web_view_load_uri
//	Reload     Reload                 / reload                / webkit_web_view_reload
//	Stop       Stop                   / stopLoading           / webkit_web_view_stop_loading
//	Back       GoBack                 / goBack                / webkit_web_view_go_back
//	Forward    GoForward              / goForward             / webkit_web_view_go_forward
//	SetBounds  Controller.put_Bounds  / frame                 / gtk_widget_size_allocate
type View interface {
	Navigate(url string) error
	Reload() error
	Stop() error
	Back() error
	Forward() error
	// SetTopInset re-lays the view out to fill its host window below top,
	// measured in physical pixels from the top of the client area.
	//
	// The view measures the window itself rather than being handed a size. The
	// host reports geometry in device-independent pixels while the native view
	// wants physical ones, and having one side convert for the other is how the
	// two end up disagreeing on a scaled display.
	SetTopInset(top int) error
	// OpenDevTools opens the engine's inspector for this view. Available only
	// where the build enabled it.
	OpenDevTools() error
	// Close releases the view and its native resources.
	Close()
}

// stateFor builds a ViewState with the trust fields derived from the URL.
func stateFor(platform Platform, url, title string, loading, canBack, canFwd bool) ViewState {
	trust := Classify(platform, url)
	return ViewState{
		URL:     url,
		Title:   title,
		Loading: loading,
		CanBack: canBack,
		CanFwd:  canFwd,
		Trusted: trust.Trusted,
		Secure:  trust.Secure,
		Host:    trust.Host,
	}
}
