package serverpicker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/actionlog"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// PingEvent carries one POP's measurement to the UI as it lands, so a 40-row
// table fills in progressively instead of sitting blank for the whole sweep.
const PingEvent = "serverpicker:ping"

// PingDoneEvent fires once a sweep finishes or is superseded.
const PingDoneEvent = "serverpicker:ping-done"

const (
	sdrFetchTimeout = 20 * time.Second
	// pingConcurrency bounds how many POPs are measured at once. Each in-flight
	// echo blocks an OS thread (IcmpSendEcho is synchronous), so this is a
	// thread budget as much as a politeness limit.
	pingConcurrency = 16
)

// ServerPopDTO is one point of presence inside a group row.
type ServerPopDTO struct {
	ID     string `json:"id"`
	Desc   string `json:"desc"`
	Relays int    `json:"relays"`
}

// ServerGroupDTO is one row of the table: a place, and every POP Steam runs there.
type ServerGroupDTO struct {
	ID          string         `json:"id"`
	Label       string         `json:"label"`
	Country     string         `json:"country"`
	CountryName string         `json:"countryName"`
	Region      string         `json:"region"`
	Blocked     bool           `json:"blocked"`
	Members     []ServerPopDTO `json:"members"`
}

// ServerListDTO is everything the page needs for one game.
type ServerListDTO struct {
	GameID   string           `json:"gameId"`
	Revision int64            `json:"revision"`
	Elevated bool             `json:"elevated"`
	Groups   []ServerGroupDTO `json:"groups"`
}

type Service struct {
	mu      sync.Mutex
	loaded  map[string][]Group // game id -> groups, as last fetched
	cancels map[string]context.CancelFunc
}

func NewService() *Service {
	return &Service{
		loaded:  map[string][]Group{},
		cancels: map[string]context.CancelFunc{},
	}
}

// ServiceName is the Wails service name.
func (s *Service) ServiceName() string { return "ServerPickerService" }

// Games lists the entries of the game dropdown.
func (s *Service) Games() ([]GameDTO, error) {
	if err := security.RequireUnlocked(); err != nil {
		return nil, err
	}
	out := make([]GameDTO, 0, len(games))
	for _, g := range games {
		out = append(out, GameDTO{ID: g.ID, Name: g.Name, AppID: g.AppID})
	}
	return out, nil
}

// IsElevated reports whether this process can write firewall rules.
func (s *Service) IsElevated() (bool, error) {
	if err := security.RequireUnlocked(); err != nil {
		return false, err
	}
	return winutil.IsProcessElevated(), nil
}

// LoadServers fetches the game's relay list and reconciles it with what the
// firewall is actually enforcing.
func (s *Service) LoadServers(gameID string) (ServerListDTO, error) {
	if err := security.RequireUnlocked(); err != nil {
		return ServerListDTO{}, err
	}
	game, ok := gameByID(gameID)
	if !ok {
		return ServerListDTO{}, fmt.Errorf("unknown game %q", gameID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), sdrFetchTimeout)
	defer cancel()
	cfg, err := fetchSDRConfig(ctx, game.AppID)
	if err != nil {
		return ServerListDTO{}, err
	}
	groups := buildGroups(cfg, game)

	blocked := s.reconcileBlocked(cfg.Revision, groups)

	s.mu.Lock()
	s.loaded[game.ID] = groups
	s.mu.Unlock()

	dto := ServerListDTO{
		GameID:   game.ID,
		Revision: cfg.Revision,
		Elevated: winutil.IsProcessElevated(),
		Groups:   make([]ServerGroupDTO, 0, len(groups)),
	}
	for _, g := range groups {
		members := make([]ServerPopDTO, 0, len(g.Members))
		for _, m := range g.Members {
			members = append(members, ServerPopDTO{ID: m.ID, Desc: m.Desc, Relays: len(m.Relay)})
		}
		_, isBlocked := blocked[g.ID]
		dto.Groups = append(dto.Groups, ServerGroupDTO{
			ID:          g.ID,
			Label:       g.Label,
			Country:     g.Country,
			CountryName: g.Name,
			Region:      g.Region,
			Blocked:     isBlocked,
			Members:     members,
		})
	}
	return dto, nil
}

// reconcileBlocked answers "what is blocked right now". The firewall wins over
// the state file: a user who reset their firewall, or who also ran another
// picker, should see what is actually in force rather than what we last
// intended. The file is only the fallback when the rules cannot be read.
func (s *Service) reconcileBlocked(revision int64, groups []Group) map[string]struct{} {
	st, err := loadState()
	if err != nil {
		serverPickerLog.Warn("could not read saved selection", "err", err)
		st = defaultState()
	}

	ids := st.BlockedGroups
	fromFirewall, ferr := listBlockedGroupIDs()
	if ferr != nil {
		serverPickerLog.Warn("could not read firewall rules; using saved selection", "err", ferr)
	} else {
		ids = fromFirewall
	}

	blocked := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		blocked[id] = struct{}{}
	}

	// Valve reshuffles relay addresses between revisions. A rule written against
	// the old set would still exist but no longer cover the POP, so rewrite the
	// affected rules - when we can, which needs elevation.
	revisionStale := revision != 0 && st.SDRRevision != revision
	if revisionStale && ferr == nil && winutil.IsProcessElevated() {
		for _, g := range groups {
			if _, on := blocked[g.ID]; !on {
				continue
			}
			if err := applyBlock(g.ID, g.RelayIPs(), true); err != nil {
				serverPickerLog.Warn("could not refresh firewall rule", "group", g.ID, "err", err)
				revisionStale = false
			}
		}
	}

	next := State{BlockedGroups: ids, SDRRevision: st.SDRRevision}
	if revisionStale && ferr == nil && winutil.IsProcessElevated() {
		next.SDRRevision = revision
	}
	if err := saveState(next); err != nil {
		serverPickerLog.Warn("could not save selection", "err", err)
	}
	return blocked
}

// SetGroupBlocked blocks or unblocks one group. Blocked means Steam cannot reach
// any of the group's relays, so matchmaking skips those servers.
func (s *Service) SetGroupBlocked(gameID, groupID string, blocked bool) error {
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	return s.setBlocked(gameID, []string{groupID}, blocked)
}

// SetManyBlocked applies one state to several groups, which is what the
// Disable All / Enable All button does over the filtered rows.
func (s *Service) SetManyBlocked(gameID string, groupIDs []string, blocked bool) error {
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	return s.setBlocked(gameID, groupIDs, blocked)
}

func (s *Service) setBlocked(gameID string, groupIDs []string, blocked bool) error {
	if !winutil.IsProcessElevated() {
		return winutil.NewNeedsAdminError("")
	}

	s.mu.Lock()
	groups := s.loaded[strings.TrimSpace(strings.ToLower(gameID))]
	s.mu.Unlock()
	if len(groups) == 0 {
		return errors.New("server list not loaded")
	}
	byID := make(map[string]Group, len(groups))
	for _, g := range groups {
		byID[g.ID] = g
	}

	op := "firewall:unblock"
	if blocked {
		op = "firewall:block"
	}
	var failures []string
	for _, raw := range groupIDs {
		id := strings.TrimSpace(raw)
		g, ok := byID[id]
		if !ok {
			continue
		}
		ips := g.RelayIPs()
		err := applyBlock(id, ips, blocked)
		actionlog.Record(op, id, strings.Join(ips, ","), err)
		if err != nil {
			if winutil.IsNeedsAdmin(err) {
				return err
			}
			serverPickerLog.Error("could not apply firewall rule", "group", id, "blocked", blocked, "err", err)
			failures = append(failures, id)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("could not update %d server(s): %s", len(failures), strings.Join(failures, ", "))
	}
	return s.persistCurrentSelection()
}

// persistCurrentSelection writes back what the firewall now holds, so the next
// launch starts from truth even if reading rules fails then.
func (s *Service) persistCurrentSelection() error {
	ids, err := listBlockedGroupIDs()
	if err != nil {
		serverPickerLog.Warn("could not read back firewall rules", "err", err)
		return nil
	}
	st, err := loadState()
	if err != nil {
		st = defaultState()
	}
	st.BlockedGroups = ids
	if err := saveState(st); err != nil {
		serverPickerLog.Warn("could not save selection", "err", err)
	}
	return nil
}

// RefreshPings measures every POP of the loaded game, emitting each result as it
// arrives. A second call supersedes the first rather than running both.
func (s *Service) RefreshPings(gameID string) error {
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	key := strings.TrimSpace(strings.ToLower(gameID))

	s.mu.Lock()
	groups := s.loaded[key]
	if cancel := s.cancels[key]; cancel != nil {
		cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancels[key] = cancel
	s.mu.Unlock()

	if len(groups) == 0 {
		cancel()
		return errors.New("server list not loaded")
	}

	var pops []POP
	for _, g := range groups {
		pops = append(pops, g.Members...)
	}

	go func() {
		defer cancel()
		sem := make(chan struct{}, pingConcurrency)
		var wg sync.WaitGroup
		for _, pop := range pops {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(p POP) {
				defer wg.Done()
				defer func() { <-sem }()
				res := measurePOP(ctx, p)
				if ctx.Err() != nil {
					return
				}
				emit(PingEvent, res)
			}(pop)
		}
		wg.Wait()
		if ctx.Err() == nil {
			emit(PingDoneEvent, key)
		}
	}()
	return nil
}

func emit(name string, payload any) {
	app := application.Get()
	if app == nil {
		return
	}
	_ = app.Event.Emit(name, payload)
}
