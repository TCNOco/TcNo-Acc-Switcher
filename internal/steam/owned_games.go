package steam

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/steam/ownedgames"
)

// ownedGamesIconWarmTimeout bounds the background artwork pass. A library of a
// few thousand apps at five concurrent downloads is minutes of work, and it must
// not outlive the view that asked for it by much.
const ownedGamesIconWarmTimeout = 10 * time.Minute

// OwnedGameDTO is one game in the games view, with the accounts known to own it.
type OwnedGameDTO struct {
	AppID   string `json:"appId"`
	Name    string `json:"name"`
	IconURL string `json:"iconUrl"`
	// Owners is empty for a game that is installed locally but owned by no vault
	// account. That means ownership is unknown, not that nobody owns it: a
	// library can only be read while the Steam Guard vault is unlocked.
	Owners []string `json:"owners"`
}

var (
	ownedGamesSweepMu   sync.RWMutex
	ownedGamesSweepHook func()
)

// SetOwnedGamesSweepHook wires the Steam Guard sweep to RefreshOwnedGames.
//
// The sweep needs an unlocked vault, which lives in internal/steamguard - and
// steamguard already imports this package, so the trigger has to travel back
// through a hook rather than an import. Pass nil at shutdown.
func SetOwnedGamesSweepHook(fn func()) {
	ownedGamesSweepMu.Lock()
	ownedGamesSweepHook = fn
	ownedGamesSweepMu.Unlock()
}

// ownedGamesWarming keeps a repeated refresh from stacking artwork passes on top
// of each other; each one spawns its own bounded fan-out.
var ownedGamesWarming atomic.Bool

// These two are vars for the same reason gameIconCacheDir is: both reach past
// the process, one into whatever Steam is installed on the machine and one onto
// the network, and neither belongs in a unit test.
var (
	ownedGamesInstalledFn  = installedGamesForOwnedList
	ownedGamesWarmFn       = WarmGameIcons
	ownedGamesLocalIconsFn = EnsureLocalGameIcons
)

// GetOwnedGamesList joins every vault account's stored library into one list of
// games, with the accounts that own each.
//
// It reads the store only: no vault access, no credentials, no network beyond
// the artwork warmed in the background. A locked vault is a non-event here -
// locking stops the sweep refreshing, but the last known libraries still render.
func (s *SteamService) GetOwnedGamesList() ([]OwnedGameDTO, error) {
	if err := security.RequireUnlocked(); err != nil {
		return nil, err
	}

	entries, err := ownedgames.Load()
	if err != nil {
		// An unreadable store must not hide the installed games as well.
		steamLog.Warn("steam owned games store unreadable", slog.Any("err", err))
		entries = map[string]ownedgames.Entry{}
	}
	owners := make(map[string][]string)
	for steamID64, entry := range entries {
		for _, appID := range entry.AppIDs {
			id := strconv.FormatUint(uint64(appID), 10)
			owners[id] = append(owners[id], steamID64)
		}
	}

	// Resolved once for the whole screen: the map is a copy of the entire Steam
	// catalogue, and the installed-games half needs the same one.
	names, err := ensureAppNameMap(context.Background())
	if err != nil {
		names = map[string]string{}
	}

	list := make([]OwnedGameDTO, 0, len(owners))
	for appID, ids := range owners {
		sort.Strings(ids)
		list = append(list, OwnedGameDTO{
			AppID:  appID,
			Name:   ownedGameName(names, appID),
			Owners: ids,
		})
	}
	for _, installed := range ownedGamesInstalledFn(names) {
		if _, owned := owners[installed.AppID]; owned {
			continue
		}
		list = append(list, OwnedGameDTO{
			AppID:  installed.AppID,
			Name:   installed.Name,
			Owners: []string{},
		})
	}
	applyLocalGameIcons(list)
	sortOwnedGames(list)

	startOwnedGamesIconWarm(list)
	return list, nil
}

// sortOwnedGames orders the list by name, case-insensitively, and by app id
// where two games share a name.
//
// The lowercase key is computed once per game rather than inside the comparator.
// A vault of a dozen accounts joins to around twenty thousand rows, so
// lowercasing on both sides of every comparison costs roughly half a million
// allocations to produce a few thousand distinct keys - measured at 43ms against
// 6.7ms for one pass up front.
func sortOwnedGames(list []OwnedGameDTO) {
	type keyed struct {
		name string
		game OwnedGameDTO
	}
	keys := make([]keyed, len(list))
	for i, game := range list {
		keys[i] = keyed{name: strings.ToLower(game.Name), game: game}
	}
	slices.SortFunc(keys, func(a, b keyed) int {
		if a.name != b.name {
			return strings.Compare(a.name, b.name)
		}
		return strings.Compare(a.game.AppID, b.game.AppID)
	})
	for i, entry := range keys {
		list[i] = entry.game
	}
}

// installedGamesForOwnedList lists the games on this machine, named from the map
// the caller already resolved. A missing or unreadable Steam install is not an
// error here: the stored libraries stand on their own.
func installedGamesForOwnedList(names map[string]string) []InstalledGameInfo {
	root, err := installRoot()
	if err != nil || strings.TrimSpace(root) == "" {
		return nil
	}
	installed, err := buildInstalledGamesListWithNames(root, names)
	if err != nil {
		steamLog.Debug("steam installed games unavailable for owned games list", slog.Any("err", err))
		return nil
	}
	return installed
}

// ownedGameName matches what BuildInstalledGamesList does for an app the name
// map has never heard of, so the two halves of the list read alike.
func ownedGameName(names map[string]string, appID string) string {
	if name := strings.TrimSpace(names[appID]); name != "" {
		return name
	}
	return "App " + appID
}

// applyLocalGameIcons fills in every icon that resolves from disk, and leaves
// the rest empty.
//
// GameIconURL only builds a path. Handing one out for a file that is not there
// yet is what made the whole list render as broken images: the background warm
// mixes instant librarycache copies with CDN requests for the app ids that have
// no local art, so on a real library it had cached 51 of 4139 available icons
// while every row already pointed at a 404. The local pass is disk-bound and
// measured in hundreds of milliseconds for a library of this size, so it runs
// here, before the first paint. A row with no icon yet keeps an empty URL and
// the view draws its placeholder until the background pass fetches one.
func applyLocalGameIcons(list []OwnedGameDTO) {
	if len(list) == 0 {
		return
	}
	appIDs := make([]string, 0, len(list))
	for _, game := range list {
		appIDs = append(appIDs, game.AppID)
	}
	icons := ownedGamesLocalIconsFn(appIDs)
	for i := range list {
		list[i].IconURL = icons[list[i].AppID]
	}
}

// startOwnedGamesIconWarm caches artwork off the UI path, for the app ids
// applyLocalGameIcons could not resolve from disk. Those need the CDN, so the
// view paints without them and fills in on its next repaint.
func startOwnedGamesIconWarm(list []OwnedGameDTO) {
	if len(list) == 0 || !ownedGamesWarming.CompareAndSwap(false, true) {
		return
	}
	appIDs := make([]string, 0, len(list))
	for _, game := range list {
		appIDs = append(appIDs, game.AppID)
	}
	go func() {
		defer crashlog.Capture()
		defer ownedGamesWarming.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), ownedGamesIconWarmTimeout)
		defer cancel()
		ownedGamesWarmFn(ctx, appIDs)
	}()
}

// RefreshOwnedGames asks the Steam Guard sweep to re-read every account's
// library now. It returns as soon as the sweep is signalled: the work itself is
// paced across accounts and reports back through OwnedGamesUpdatedEvent.
func (s *SteamService) RefreshOwnedGames() error {
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	ownedGamesSweepMu.RLock()
	hook := ownedGamesSweepHook
	ownedGamesSweepMu.RUnlock()
	if hook == nil {
		return errors.New("steam owned games refresh is unavailable")
	}
	hook()
	return nil
}
