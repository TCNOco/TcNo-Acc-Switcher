package steambrowser

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
)

// MaxWindows bounds how many session windows can be open at once. Each one costs
// a WebView2 browser process group, so an accidental loop opening them would
// otherwise be felt across the machine rather than just in this app.
const MaxWindows = 8

var (
	// ErrTooManyWindows reports the cap being reached.
	ErrTooManyWindows = fmt.Errorf("steambrowser: at most %d session windows can be open at once", MaxWindows)
	// ErrNoSuchWindow reports a command for a window that has already closed.
	ErrNoSuchWindow = errors.New("steambrowser: session window is not open")
)

// Site is one of the destinations a session window can be opened on.
//
// The set is closed, and the frontend names a member of it rather than a URL or
// an app id. That is what keeps a value crossing the boundary from choosing the
// page: an unknown site is refused here, so nothing outside this file can send a
// window carrying an account's session somewhere of its own choosing.
type Site string

const (
	SiteStore     Site = "store"
	SiteCommunity Site = "community"
	SiteChat      Site = "chat"
	// Steam publishes a Personal Game Data page per game, but only for a handful
	// of its own titles. One site each, rather than a site taking an app id,
	// because these three are the whole list.
	SiteGameDataCS2   Site = "gamedata-730"
	SiteGameDataTF2   Site = "gamedata-440"
	SiteGameDataDota2 Site = "gamedata-570"
)

// Destination is the landing page for a site. Community deliberately lands on
// the account's own profile rather than the site root, which is the page the
// user asked for when they picked the account.
func (s Site) Destination(steamID64 string) (string, error) {
	switch s {
	case SiteStore:
		return "https://store.steampowered.com/", nil
	case SiteChat:
		return "https://steamcommunity.com/chat/", nil
	case SiteCommunity:
		return profilePage(steamID64, "")
	case SiteGameDataCS2:
		return profilePage(steamID64, "gcpd/730/")
	case SiteGameDataTF2:
		return profilePage(steamID64, "gcpd/440/")
	case SiteGameDataDota2:
		return profilePage(steamID64, "gcpd/570/")
	default:
		return "", fmt.Errorf("steambrowser: unknown site %q", s)
	}
}

// profilePage builds a page under the account's own profile. The Steam ID is
// checked here rather than at each call site because this is where it is
// interpolated into a URL.
func profilePage(steamID64, suffix string) (string, error) {
	if !validSteamID64(steamID64) {
		return "", fmt.Errorf("%w: bad Steam ID", ErrInvalidSession)
	}
	return "https://steamcommunity.com/profiles/" + steamID64 + "/" + suffix, nil
}

// session is one open window: its content view, and the identity it belongs to.
//
// It holds no vault handle. The session is fully materialised into cookies when
// the window opens, so re-locking the vault afterwards leaves open windows
// working, which is the behaviour a browser has and what avoids yanking a window
// away mid-purchase.
type session struct {
	id        string
	steamID64 string
	account   string
	site      Site
	platform  Platform
	// profile is the storage this window's account uses. A second window onto
	// the same account names it again and inherits the planted session.
	profile string
	view    View
	state   ViewState
}

// registry tracks the open windows. Several may belong to the same account, and
// several accounts may be open at once; the profile on each view is what keeps
// their sessions apart.
type registry struct {
	mu       sync.Mutex
	sessions map[string]*session
}

func newRegistry() *registry {
	return &registry{sessions: map[string]*session{}}
}

// reserve allocates an id for a window that is about to open, so the cap is
// enforced before the expensive work of creating a view begins.
func (r *registry) reserve() (string, error) {
	id, err := newSessionKey()
	if err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.sessions) >= MaxWindows {
		return "", ErrTooManyWindows
	}
	// A placeholder holds the slot; add replaces it once the view exists.
	r.sessions[id] = nil
	return id, nil
}

func (r *registry) add(s *session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.id] = s
}

// release drops a reservation that never became a window.
func (r *registry) release(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, id)
}

func (r *registry) get(id string) (*session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok || s == nil {
		return nil, ErrNoSuchWindow
	}
	return s, nil
}

// remove takes a window out of the registry and returns it, so the caller can
// close the view exactly once.
func (r *registry) remove(id string) *session {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.sessions[id]
	delete(r.sessions, id)
	return s
}

func (r *registry) setState(id string, state ViewState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s := r.sessions[id]; s != nil {
		s.state = state
	}
}

func (r *registry) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sessions)
}

// ids returns the open window ids, for shutting them all down.
func (r *registry) ids() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.sessions))
	for id, s := range r.sessions {
		if s != nil {
			out = append(out, id)
		}
	}
	return out
}

func newSessionKey() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("steambrowser: generate window id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
