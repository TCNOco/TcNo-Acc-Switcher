package steambrowser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// StateEvent is emitted to a session window whenever its page changes.
const StateEvent = "steambrowser:state"

// defaultChromeHeight is how far down the window the page starts, in
// device-independent pixels, used until the window's own chrome measures itself.
// It has to clear the application's title bar as well as the toolbar, so it is
// the toolbar's bottom edge rather than its height.
const defaultChromeHeight = 76

// ErrNeedsLogin reports an account whose stored session can no longer be
// renewed. It is an expected outcome rather than a failure, so OpenBrowser
// turns it into a result the caller can act on: the answer is to offer the
// sign-in screen, not to show an error.
var ErrNeedsLogin = errors.New("steambrowser: the account must sign in again")

// OpenResult is what opening a window produced.
type OpenResult struct {
	// SessionID names the new window. Empty when NeedsLogin is set.
	SessionID string `json:"sessionId"`
	// NeedsLogin asks the caller to put the user through the sign-in screen.
	NeedsLogin bool `json:"needsLogin"`
}

// SessionSource hands out a usable web session for an account.
//
// It is an interface so this package does not depend on the Steam Guard vault,
// which keeps trust, session and window logic testable without one. The Steam
// Guard service implements it.
type SessionSource interface {
	// BrowserSession returns the credentials for accountID, after checking the
	// modal token exactly as any other sensitive Steam Guard view would. It is
	// responsible for refreshing a lapsed session before returning.
	BrowserSession(accountID, modalToken string) (WebSession, error)
}

// WebSession is one account's Steam web credentials.
type WebSession struct {
	SteamID64   string
	AccountName string
	AccessToken string
	SessionID   string
}

// Service opens and drives the session browser windows.
type Service struct {
	sessions *registry
	source   SessionSource
	dataPath string
	devTools bool

	mu      sync.Mutex
	heights map[string]int // session id -> measured chrome height, in DIP
}

// NewService builds the service. dataPath is the user data folder every
// account's profile lives under.
func NewService(source SessionSource, dataPath string, devTools bool) *Service {
	return &Service{
		sessions: newRegistry(),
		source:   source,
		dataPath: dataPath,
		devTools: devTools,
		heights:  map[string]int{},
	}
}

// ServiceName is the Wails service name.
func (s *Service) ServiceName() string { return "SteamBrowserService" }

// Available reports whether this build can open session windows, so the UI can
// hide the entry points rather than offering something that always fails.
func (s *Service) Available() bool { return Supported() }

// OpenBrowser opens a window signed in as accountID and returns its id.
//
// The session is fully materialised into cookies here. Once the window is open
// it holds no vault handle, so re-locking the vault leaves it working.
func (s *Service) OpenBrowser(accountID, site, modalToken string) (OpenResult, error) {
	if !Supported() {
		return OpenResult{}, ErrUnsupportedPlatform
	}
	if s.source == nil {
		return OpenResult{}, errors.New("steambrowser: no session source")
	}
	accountID = strings.TrimSpace(accountID)

	log := logger().With("steamId64", accountID, "site", site)
	log.Info("opening a session browser window")

	session, err := s.source.BrowserSession(accountID, modalToken)
	if errors.Is(err, ErrNeedsLogin) {
		// Not a failure: the account simply has no session left to browse with.
		log.Info("account must sign in again before a window can open")
		return OpenResult{NeedsLogin: true}, nil
	}
	if err != nil {
		log.Warn("no usable session for this account", "error", err)
		return OpenResult{}, err
	}
	destination, err := Site(site).Destination(session.SteamID64)
	if err != nil {
		return OpenResult{}, err
	}
	cookies, err := SessionCookies(session.SteamID64, session.AccessToken, session.SessionID)
	if err != nil {
		return OpenResult{}, err
	}
	profile, err := ProfileName(session.SteamID64)
	if err != nil {
		return OpenResult{}, err
	}

	id, err := s.sessions.reserve()
	if err != nil {
		return OpenResult{}, err
	}

	// Window creation and the content view both touch UI-thread-only state: Wails
	// windows must be made on the main thread, and WebView2 is COM bound to the
	// apartment set up there. Doing the whole sequence in one dispatch also keeps
	// the view from being created against a window that has since gone.
	var openErr error
	application.InvokeSync(func() {
		openErr = s.openOnMainThread(id, session, Site(site), destination, profile, cookies)
	})
	if openErr != nil {
		s.sessions.release(id)
		log.Error("could not open the session browser window", "window", id, "error", openErr)
		return OpenResult{}, openErr
	}
	log.Info("session browser window open", "window", id, "open", s.sessions.count())
	return OpenResult{SessionID: id}, nil
}

func (s *Service) openOnMainThread(id string, credentials WebSession, site Site, destination, profile string, cookies []Cookie) error {
	app := application.Get()
	if app == nil || app.Window == nil {
		return errors.New("steambrowser: application is not running")
	}

	window := app.Window.NewWithOptions(chromeWindowOptions(id, windowTitle(credentials.AccountName, site)))
	nativeWindow := uintptr(window.NativeWindow())
	if nativeWindow == 0 {
		window.Close()
		return errors.New("steambrowser: host window has no native handle")
	}
	logger().Debug("host window created", "window", id, "hwnd", nativeWindow, "destination", destination)

	view, err := newView(ViewOptions{
		NativeWindow: nativeWindow,
		Profile:      profile,
		DataPath:     s.dataPath,
		DevTools:     s.devTools,
		ReservedTop:  scaleForWindow(window, defaultChromeHeight),
		Cookies:      cookies,
		InitialURL:   destination,
		Platform:     PlatformSteam,
		OnState:      func(state ViewState) { s.publish(id, state) },
		OnNewWindow:  func(url string) { s.handleNewWindow(id, url) },
	})
	if err != nil {
		window.Close()
		return err
	}

	s.sessions.add(&session{
		id:        id,
		steamID64: credentials.SteamID64,
		account:   credentials.AccountName,
		site:      site,
		platform:  PlatformSteam,
		profile:   profile,
		view:      view,
	})

	window.OnWindowEvent(events.Common.WindowDidResize, func(*application.WindowEvent) {
		s.layout(id)
	})
	window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		closing := s.sessions.remove(id)
		s.mu.Lock()
		delete(s.heights, id)
		s.mu.Unlock()
		if closing == nil || closing.view == nil {
			return
		}
		// Releasing the view is a COM call, so it belongs on the main thread for
		// the same reason layout does.
		view := closing.view
		application.InvokeAsync(view.Close)
	})
	s.layout(id)
	return nil
}

// SetChromeHeight lets a window's toolbar report where the page should start,
// in CSS pixels measured from the top of the window.
//
// It is the toolbar's bottom edge, not its height, because the application's
// own title bar sits above it. Measuring rather than assuming also keeps the
// content view in place when the theme or font size moves the toolbar.
func (s *Service) SetChromeHeight(sessionID string, height int) error {
	if height <= 0 || height > 400 {
		return fmt.Errorf("steambrowser: implausible chrome height %d", height)
	}
	s.mu.Lock()
	s.heights[sessionID] = height
	s.mu.Unlock()

	s.layout(sessionID)
	return nil
}

// layout puts the content view below the window's chrome. Only the inset is
// converted here; the view measures the window's own client area, which avoids
// both sides having to agree on device-independent versus physical pixels.
//
// The work is queued onto the main thread rather than run here. The native view
// is COM and refuses calls from any other thread, and layout is reached from
// three places on two different threads: window creation is already on the main
// thread, while a resize event and the toolbar's own measurement are not.
// InvokeAsync is the one that is safe from both — InvokeSync blocks until the
// main thread runs the callback, which is a deadlock when the caller is the main
// thread.
func (s *Service) layout(sessionID string) {
	application.InvokeAsync(func() {
		current, err := s.sessions.get(sessionID)
		if err != nil || current.view == nil {
			return
		}
		app := application.Get()
		if app == nil || app.Window == nil {
			return
		}
		window, ok := app.Window.GetByName(windowName(sessionID))
		if !ok || window == nil {
			return
		}
		s.mu.Lock()
		height := s.heights[sessionID]
		s.mu.Unlock()
		if height <= 0 {
			height = defaultChromeHeight
		}
		if err := current.view.SetTopInset(scaleForWindow(window, height)); err != nil {
			logger().Warn("could not lay the content view out", "window", sessionID, "error", err)
		}
	})
}

func (s *Service) publish(sessionID string, state ViewState) {
	s.sessions.setState(sessionID, state)
	logger().Debug("page changed",
		"window", sessionID, "host", state.Host,
		"trusted", state.Trusted, "loading", state.Loading)
	app := application.Get()
	if app == nil || app.Window == nil {
		return
	}
	window, ok := app.Window.GetByName(windowName(sessionID))
	if !ok || window == nil {
		return
	}
	window.DispatchWailsEvent(&application.CustomEvent{
		Name:   StateEvent,
		Sender: "native",
		Data:   state,
	})
}

// handleNewWindow routes a middle-click or popup. A trusted target opens as
// another session window for the same account; anything else goes to the user's
// own browser, so a page cannot pull an untrusted site into a window that is
// wearing this account's session.
func (s *Service) handleNewWindow(sessionID, url string) {
	current, err := s.sessions.get(sessionID)
	if err != nil {
		return
	}
	if !IsTrusted(current.platform, url) {
		logger().Info("handing an untrusted link to the system browser",
			"window", sessionID, "host", Classify(current.platform, url).Host)
		if app := application.Get(); app != nil && app.Browser != nil {
			_ = app.Browser.OpenURL(url)
		}
		return
	}
	// Reuse this window's own account rather than asking the vault again, which
	// would need a modal token the page cannot have.
	go func() {
		if _, err := s.openLinked(current, url); err != nil {
			if app := application.Get(); app != nil && app.Browser != nil {
				_ = app.Browser.OpenURL(url)
			}
		}
	}()
}

// Navigate sends a window to a URL the user typed or dropped. A value with no
// scheme is treated as https, and anything that is not http(s) is refused so a
// typed file: or javascript: URL cannot be run in a session window.
func (s *Service) Navigate(sessionID, rawURL string) error {
	target, err := normaliseTypedURL(rawURL)
	if err != nil {
		return err
	}
	return s.command(sessionID, func(v View) error { return v.Navigate(target) })
}

func (s *Service) Back(sessionID string) error {
	return s.command(sessionID, View.Back)
}

func (s *Service) Forward(sessionID string) error {
	return s.command(sessionID, View.Forward)
}

func (s *Service) Reload(sessionID string) error {
	return s.command(sessionID, View.Reload)
}

func (s *Service) Stop(sessionID string) error {
	return s.command(sessionID, View.Stop)
}

// State returns the last reported state, for a toolbar that has just mounted and
// missed the events so far.
func (s *Service) State(sessionID string) (ViewState, error) {
	current, err := s.sessions.get(sessionID)
	if err != nil {
		return ViewState{}, err
	}
	return current.state, nil
}

// Certificate describes the certificate for a window's current page.
func (s *Service) Certificate(ctx context.Context, sessionID string) (Certificate, error) {
	current, err := s.sessions.get(sessionID)
	if err != nil {
		return Certificate{}, err
	}
	return FetchCertificate(ctx, current.state.URL)
}

// TrustedDomains lets the UI explain what the green URL bar means.
func (s *Service) TrustedDomains() []string {
	return TrustedDomains(PlatformSteam)
}

// CloseAll shuts every session window, for application shutdown.
func (s *Service) CloseAll() {
	for _, id := range s.sessions.ids() {
		if closing := s.sessions.remove(id); closing != nil && closing.view != nil {
			view := closing.view
			application.InvokeSync(func() { view.Close() })
		}
	}
}

// command runs an operation against a window's view on the UI thread.
func (s *Service) command(sessionID string, run func(View) error) error {
	current, err := s.sessions.get(sessionID)
	if err != nil {
		return err
	}
	if current.view == nil {
		return ErrNoSuchWindow
	}
	var runErr error
	application.InvokeSync(func() { runErr = run(current.view) })
	return runErr
}

// openLinked opens another window onto an account that already has one, which is
// how a middle-click on a trusted link becomes a window rather than a popup.
//
// It asks the vault for nothing. The account's cookies are already in its
// profile's jar from the window this link came from, so the new view inherits
// the session by naming the same profile. That keeps the modal token as the only
// way to start a session: a page can cause another window onto an account the
// user already opened, never onto one they did not.
func (s *Service) openLinked(from *session, url string) (string, error) {
	id, err := s.sessions.reserve()
	if err != nil {
		return "", err
	}
	credentials := WebSession{SteamID64: from.steamID64, AccountName: from.account}
	var openErr error
	application.InvokeSync(func() {
		openErr = s.openOnMainThread(id, credentials, from.site, url, from.profile, nil)
	})
	if openErr != nil {
		s.sessions.release(id)
		return "", openErr
	}
	return id, nil
}
