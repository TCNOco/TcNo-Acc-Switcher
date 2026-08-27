package steam

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/steam/ownedgames"
)

// Scale measured on the developer's own machine: ~4856 owned app ids per account
// once DLC is counted, 7-12 accounts in a vault, 5947 apps in the local library
// cache, and an AppIdsUser.json holding the whole Steam catalogue (180898
// entries at the time of writing).
//
// benchSharedAppIDs models the overlap between one person's alt accounts: they
// buy most things once and share the bulk of the library, so the union across
// the vault is far smaller than accounts*perAccount.
const (
	benchVaultAccounts  = 12
	benchAppIDsPerAcct  = 4856
	benchSharedAppIDs   = 4000
	benchInstalledApps  = 5947
	benchNameMapEntries = 180898
	benchAppIDSpace     = 3_000_000
)

type ownedGamesBenchData struct {
	accounts  []ownedgames.Entry
	names     map[string]string
	installed []InstalledGameInfo
	// union is the number of distinct app ids owned across the whole vault -
	// the real width of the join, and the figure the benchmarks are read against.
	union int
}

// benchStoreFile mirrors the on-disk shape of the owned games store so the
// benchmark can lay down a full vault in one write instead of paying
// ownedgames.Put's load-mutate-save once per account.
type benchStoreFile struct {
	Version int                `json:"version"`
	Entries []ownedgames.Entry `json:"entries"`
}

var benchNameWords = []string{
	"Counter", "Strike", "Global", "Offensive", "Dota", "Half", "Life", "Portal",
	"Team", "Fortress", "Left", "Dead", "Alyx", "Deathmatch", "Classic", "Source",
	"Elder", "Scrolls", "Skyrim", "Fallout", "Doom", "Eternal", "Quake", "Rage",
	"Civilization", "Stellaris", "Hearts", "Iron", "Crusader", "Kings", "Europa",
	"Rust", "Ark", "Survival", "Evolved", "Subnautica", "Terraria", "Starbound",
	"Deluxe", "Edition", "Remastered", "Definitive", "Anniversary", "Soundtrack",
	"Season", "Pass", "Expansion", "Bundle", "Demo", "Dedicated", "Server", "SDK",
	"Wallpaper", "Engine", "Garry", "Mod", "Killing", "Floor", "Payday", "Heist",
}

func newOwnedGamesBenchData() *ownedGamesBenchData {
	rng := rand.New(rand.NewPCG(0x5EA1, 0xC0DE))

	pick := func(seen map[uint32]struct{}, n int) []uint32 {
		out := make([]uint32, 0, n)
		for len(out) < n {
			id := uint32(rng.IntN(benchAppIDSpace-10)) + 10
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
		return out
	}

	taken := make(map[uint32]struct{}, benchNameMapEntries)
	shared := pick(taken, benchSharedAppIDs)

	accounts := make([]ownedgames.Entry, 0, benchVaultAccounts)
	union := make(map[uint32]struct{}, benchSharedAppIDs*4)
	for _, id := range shared {
		union[id] = struct{}{}
	}
	now := time.Now().Unix()
	for i := range benchVaultAccounts {
		ids := append([]uint32(nil), shared...)
		own := pick(taken, benchAppIDsPerAcct-benchSharedAppIDs)
		for _, id := range own {
			union[id] = struct{}{}
		}
		ids = append(ids, own...)
		rng.Shuffle(len(ids), func(a, b int) { ids[a], ids[b] = ids[b], ids[a] })
		slices.Sort(ids)
		accounts = append(accounts, ownedgames.Entry{
			SteamID64: strconv.FormatUint(76561198000000000+uint64(i), 10),
			AppIDs:    ids,
			CheckedAt: now,
		})
	}

	// The local library overlaps the vault heavily but is not a subset: a machine
	// can have games installed that no vault account has been swept for.
	installedIDs := make([]uint32, 0, benchInstalledApps)
	for _, id := range shared[:min(len(shared), benchInstalledApps*2/3)] {
		installedIDs = append(installedIDs, id)
	}
	installedIDs = append(installedIDs, pick(taken, benchInstalledApps-len(installedIDs))...)

	names := make(map[string]string, benchNameMapEntries)
	addName := func(id uint32) {
		key := strconv.FormatUint(uint64(id), 10)
		if _, ok := names[key]; ok {
			return
		}
		names[key] = benchGameName(rng)
	}
	for id := range union {
		addName(id)
	}
	for _, id := range installedIDs {
		addName(id)
	}
	for len(names) < benchNameMapEntries {
		addName(uint32(rng.IntN(benchAppIDSpace-10)) + 10)
	}

	installed := make([]InstalledGameInfo, 0, len(installedIDs))
	for _, id := range installedIDs {
		key := strconv.FormatUint(uint64(id), 10)
		installed = append(installed, InstalledGameInfo{AppID: key, Name: names[key]})
	}

	return &ownedGamesBenchData{accounts: accounts, names: names, installed: installed, union: len(union)}
}

// benchGameName produces names averaging ~17 characters with mixed case, which
// is what the real catalogue looks like and what decides whether the sort
// comparator's strings.ToLower allocates.
func benchGameName(rng *rand.Rand) string {
	var b strings.Builder
	for i, n := 0, 2+rng.IntN(3); i < n; i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(benchNameWords[rng.IntN(len(benchNameWords))])
	}
	if rng.IntN(4) == 0 {
		b.WriteByte(' ')
		b.WriteString(strconv.Itoa(rng.IntN(9) + 2))
	}
	return b.String()
}

func useOwnedGamesBenchRoot(b *testing.B, data *ownedGamesBenchData, installed func(map[string]string, map[string]string) []InstalledGameInfo) {
	b.Helper()
	paths.ResetForTest(b.TempDir())

	// A fresh AppIdsUser.json on disk is what keeps ensureAppNameMap from firing
	// a background download at the mirror mid-benchmark.
	if err := saveAppNameMapToDisk(data.names); err != nil {
		b.Fatal(err)
	}
	setSteamAppNameMapMemory(data.names)

	raw, err := json.Marshal(benchStoreFile{Version: ownedgames.Version, Entries: data.accounts})
	if err != nil {
		b.Fatal(err)
	}
	storePath, err := ownedgames.Path()
	if err != nil {
		b.Fatal(err)
	}
	if err := fsutil.WriteFileAtomic(storePath, append(raw, '\n'), 0o600); err != nil {
		b.Fatal(err)
	}

	installedFn, warmFn, localFn := ownedGamesInstalledFn, ownedGamesWarmFn, ownedGamesLocalIconsFn
	appInfoFn := appInfoNamesFn
	b.Cleanup(func() {
		// The artwork pass reads ownedGamesWarmFn inside its goroutine, so
		// restoring the real WarmGameIcons while one is still in flight would put
		// the CDN and the temp wwwroot back in play after the benchmark ended.
		for ownedGamesWarming.Load() {
			runtime.Gosched()
		}
		ownedGamesInstalledFn, ownedGamesWarmFn = installedFn, warmFn
		ownedGamesLocalIconsFn = localFn
		appInfoNamesFn = appInfoFn
		steamAppNameMapMu.Lock()
		steamAppNameMapMem = nil
		steamAppNameMapMu.Unlock()
	})
	ownedGamesInstalledFn = installed
	ownedGamesWarmFn = func(context.Context, []string) map[string]string { return nil }
	// Keeps the machine's own librarycache and appinfo cache out of the measurement.
	ownedGamesLocalIconsFn = func([]string) map[string]string { return nil }
	appInfoNamesFn = func() map[string]string { return nil }
}

// BenchmarkGetOwnedGamesList is the whole screen-open path with the local
// library handed over precomputed, so what it measures is the store read, the
// join, one app name map clone and the final sort.
func BenchmarkGetOwnedGamesList(b *testing.B) {
	data := newOwnedGamesBenchData()
	useOwnedGamesBenchRoot(b, data, func(map[string]string, map[string]string) []InstalledGameInfo { return data.installed })
	svc := NewSteamService()
	b.ReportMetric(float64(data.union), "ownedApps")
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		list, err := svc.GetOwnedGamesList()
		if err != nil {
			b.Fatal(err)
		}
		if len(list) == 0 {
			b.Fatal("empty list")
		}
	}
}

// BenchmarkGetOwnedGamesListWithInstalledBuild adds back what
// buildInstalledGamesListWithNames does after its VDF read: a name lookup per
// installed app and a sort, off the map the caller resolved. Only the disk parse
// of libraryfolders.vdf is missing, so this is the closest honest figure for one
// screen open.
func BenchmarkGetOwnedGamesListWithInstalledBuild(b *testing.B) {
	data := newOwnedGamesBenchData()
	ids := make([]string, 0, len(data.installed))
	for _, game := range data.installed {
		ids = append(ids, game.AppID)
	}
	useOwnedGamesBenchRoot(b, data, func(names, local map[string]string) []InstalledGameInfo {
		list := make([]InstalledGameInfo, 0, len(ids))
		for _, id := range ids {
			nm := resolveAppName(names, local, id)
			list = append(list, InstalledGameInfo{AppID: id, Name: nm})
		}
		sort.Slice(list, func(i, j int) bool {
			return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
		})
		return list
	})
	svc := NewSteamService()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := svc.GetOwnedGamesList(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOwnedGamesStoreLoad(b *testing.B) {
	data := newOwnedGamesBenchData()
	useOwnedGamesBenchRoot(b, data, func(map[string]string, map[string]string) []InstalledGameInfo { return data.installed })
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		entries, err := ownedgames.Load()
		if err != nil {
			b.Fatal(err)
		}
		if len(entries) != benchVaultAccounts {
			b.Fatalf("entries = %d", len(entries))
		}
	}
}

func BenchmarkOwnedGamesEnsureAppNameMap(b *testing.B) {
	data := newOwnedGamesBenchData()
	useOwnedGamesBenchRoot(b, data, func(map[string]string, map[string]string) []InstalledGameInfo { return data.installed })
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		m, err := ensureAppNameMap(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if len(m) != len(data.names) {
			b.Fatalf("names = %d", len(m))
		}
	}
}

func BenchmarkOwnedGamesAppNameMapLoadFromDisk(b *testing.B) {
	data := newOwnedGamesBenchData()
	useOwnedGamesBenchRoot(b, data, func(map[string]string, map[string]string) []InstalledGameInfo { return data.installed })
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		m, err := loadAppNameMapFromDisk()
		if err != nil {
			b.Fatal(err)
		}
		if len(m) != len(data.names) {
			b.Fatalf("names = %d", len(m))
		}
	}
}

func benchEntriesByID(data *ownedGamesBenchData) map[string]ownedgames.Entry {
	entries := make(map[string]ownedgames.Entry, len(data.accounts))
	for _, entry := range data.accounts {
		entries[entry.SteamID64] = entry
	}
	return entries
}

// BenchmarkOwnedGamesOwnerJoin is the loop GetOwnedGamesList runs over every
// account's app ids: the only place in the feature where per-app-id work is
// proportional to accounts*library.
func BenchmarkOwnedGamesOwnerJoin(b *testing.B) {
	data := newOwnedGamesBenchData()
	entries := benchEntriesByID(data)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		owners := make(map[string][]string)
		for steamID64, entry := range entries {
			for _, appID := range entry.AppIDs {
				id := strconv.FormatUint(uint64(appID), 10)
				owners[id] = append(owners[id], steamID64)
			}
		}
		if len(owners) != data.union {
			b.Fatalf("owners = %d, want %d", len(owners), data.union)
		}
	}
}

// BenchmarkOwnedGamesOwnerJoinPrealloc is the same join with the map sized up
// front, which is the smallest possible change to it.
func BenchmarkOwnedGamesOwnerJoinPrealloc(b *testing.B) {
	data := newOwnedGamesBenchData()
	entries := benchEntriesByID(data)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		owners := make(map[string][]string, benchAppIDsPerAcct*2)
		for steamID64, entry := range entries {
			for _, appID := range entry.AppIDs {
				id := strconv.FormatUint(uint64(appID), 10)
				owners[id] = append(owners[id], steamID64)
			}
		}
		if len(owners) != data.union {
			b.Fatalf("owners = %d, want %d", len(owners), data.union)
		}
	}
}

// BenchmarkOwnedGamesOwnerJoinUint32Keyed keys the join by the app id itself and
// formats each distinct id once at the end, instead of once per (account, app)
// pair, with the map sized up front.
func BenchmarkOwnedGamesOwnerJoinUint32Keyed(b *testing.B) {
	data := newOwnedGamesBenchData()
	entries := benchEntriesByID(data)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		byApp := make(map[uint32][]string, benchAppIDsPerAcct*2)
		for steamID64, entry := range entries {
			for _, appID := range entry.AppIDs {
				byApp[appID] = append(byApp[appID], steamID64)
			}
		}
		owners := make(map[string][]string, len(byApp))
		for appID, ids := range byApp {
			owners[strconv.FormatUint(uint64(appID), 10)] = ids
		}
		if len(owners) != data.union {
			b.Fatalf("owners = %d, want %d", len(owners), data.union)
		}
	}
}

func benchOwners(data *ownedGamesBenchData) map[string][]string {
	owners := make(map[string][]string)
	for _, entry := range data.accounts {
		for _, appID := range entry.AppIDs {
			id := strconv.FormatUint(uint64(appID), 10)
			owners[id] = append(owners[id], entry.SteamID64)
		}
	}
	return owners
}

// BenchmarkOwnedGamesBuildDTOs covers everything between the join and the sort:
// a sort.Strings over each app's owners, a name lookup and a GameIconURL call
// (which runs a regexp) per app.
func BenchmarkOwnedGamesBuildDTOs(b *testing.B) {
	data := newOwnedGamesBenchData()
	owners := benchOwners(data)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		list := make([]OwnedGameDTO, 0, len(owners))
		for appID, ids := range owners {
			sort.Strings(ids)
			list = append(list, OwnedGameDTO{
				AppID:   appID,
				Name:    ownedGameName(data.names, nil, appID),
				IconURL: GameIconURL(appID),
				Owners:  ids,
			})
		}
		if len(list) != data.union {
			b.Fatal("bad list")
		}
	}
}

// BenchmarkOwnedGamesIconURL isolates the regexp inside GameIconURL, which runs
// once per app in the list.
func BenchmarkOwnedGamesIconURL(b *testing.B) {
	data := newOwnedGamesBenchData()
	ids := make([]string, 0, len(data.installed))
	for _, game := range data.installed {
		ids = append(ids, game.AppID)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		for _, id := range ids {
			if GameIconURL(id) == "" {
				b.Fatal("bad id")
			}
		}
	}
}

func benchDTOList(data *ownedGamesBenchData) []OwnedGameDTO {
	owners := benchOwners(data)
	list := make([]OwnedGameDTO, 0, len(owners)+len(data.installed))
	for appID, ids := range owners {
		list = append(list, OwnedGameDTO{AppID: appID, Name: ownedGameName(data.names, nil, appID), Owners: ids})
	}
	for _, installed := range data.installed {
		if _, owned := owners[installed.AppID]; owned {
			continue
		}
		list = append(list, OwnedGameDTO{AppID: installed.AppID, Name: installed.Name, Owners: []string{}})
	}
	return list
}

// BenchmarkOwnedGamesFinalSortToLowerComparator is the shape sortOwnedGames
// replaced: strings.ToLower inside the comparator, so roughly 2*n*log2(n) calls
// and an allocation for each. Kept as the baseline the change is measured
// against.
func BenchmarkOwnedGamesFinalSortToLowerComparator(b *testing.B) {
	data := newOwnedGamesBenchData()
	list := benchDTOList(data)
	scratch := make([]OwnedGameDTO, len(list))
	b.ReportMetric(float64(len(list)), "rows")
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		copy(scratch, list)
		sort.Slice(scratch, func(i, j int) bool {
			left, right := strings.ToLower(scratch[i].Name), strings.ToLower(scratch[j].Name)
			if left == right {
				return scratch[i].AppID < scratch[j].AppID
			}
			return left < right
		})
	}
}

// TestSortOwnedGamesMatchesToLowerComparator pins the precomputed-key sort to
// the comparator it replaced, over a list wide enough to contain real name ties
// and exercise the app id tie-break.
func TestSortOwnedGamesMatchesToLowerComparator(t *testing.T) {
	list := benchDTOList(newOwnedGamesBenchData())
	want := append([]OwnedGameDTO(nil), list...)
	sort.Slice(want, func(i, j int) bool {
		left, right := strings.ToLower(want[i].Name), strings.ToLower(want[j].Name)
		if left == right {
			return want[i].AppID < want[j].AppID
		}
		return left < right
	})

	got := append([]OwnedGameDTO(nil), list...)
	sortOwnedGames(got)
	for i := range want {
		if got[i].AppID != want[i].AppID || got[i].Name != want[i].Name {
			t.Fatalf("row %d = %q/%q, want %q/%q", i, got[i].AppID, got[i].Name, want[i].AppID, want[i].Name)
		}
	}
}

func BenchmarkOwnedGamesFinalSort(b *testing.B) {
	data := newOwnedGamesBenchData()
	list := benchDTOList(data)
	scratch := make([]OwnedGameDTO, len(list))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		copy(scratch, list)
		sortOwnedGames(scratch)
	}
}

// BenchmarkOwnedGamesInstalledSort measures the same comparator shape inside
// BuildInstalledGamesList, which GetOwnedGamesList also pays on every screen
// open. That one was left alone: it is shared with the installed games view.
func BenchmarkOwnedGamesInstalledSort(b *testing.B) {
	data := newOwnedGamesBenchData()
	scratch := make([]InstalledGameInfo, len(data.installed))
	b.Run("toLowerComparator", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			copy(scratch, data.installed)
			sort.Slice(scratch, func(i, j int) bool {
				return strings.ToLower(scratch[i].Name) < strings.ToLower(scratch[j].Name)
			})
		}
	})
	b.Run("precomputedKeys", func(b *testing.B) {
		type keyed struct {
			name string
			game InstalledGameInfo
		}
		keys := make([]keyed, len(data.installed))
		b.ReportAllocs()
		for b.Loop() {
			for i, game := range data.installed {
				keys[i] = keyed{name: strings.ToLower(game.Name), game: game}
			}
			slices.SortFunc(keys, func(a, c keyed) int { return strings.Compare(a.name, c.name) })
		}
	})
}

// benchDedupeSorted replicates ownedgames.dedupeSorted so the sweep's per-account
// sort can be measured without importing an unexported function.
func benchDedupeSorted(appIDs []uint32) []uint32 {
	out := append([]uint32(nil), appIDs...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	end := 0
	for i, id := range out {
		if i > 0 && id == out[end-1] {
			continue
		}
		out[end] = id
		end++
	}
	return out[:end]
}

func benchUnsortedAppIDs() []uint32 {
	rng := rand.New(rand.NewPCG(7, 11))
	ids := make([]uint32, benchAppIDsPerAcct)
	for i := range ids {
		ids[i] = uint32(rng.IntN(benchAppIDSpace))
	}
	return ids
}

// BenchmarkOwnedGamesDedupeSorted is the sweep's per-account normalisation: one
// call per account per fetch, and the fetch is floored at six hours per account.
func BenchmarkOwnedGamesDedupeSorted(b *testing.B) {
	ids := benchUnsortedAppIDs()
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if len(benchDedupeSorted(ids)) == 0 {
			b.Fatal("empty")
		}
	}
}

// BenchmarkOwnedGamesSortUint32 sizes the one genuinely data-parallel operation
// in the feature, against the cheapest scalar improvement available.
func BenchmarkOwnedGamesSortUint32(b *testing.B) {
	ids := benchUnsortedAppIDs()
	scratch := make([]uint32, len(ids))
	b.Run("sortSlice", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			copy(scratch, ids)
			sort.Slice(scratch, func(i, j int) bool { return scratch[i] < scratch[j] })
		}
	})
	b.Run("slicesSort", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			copy(scratch, ids)
			slices.Sort(scratch)
		}
	})
}

// BenchmarkOwnedGamesUnionUint32 is the join expressed as the shape SIMD would
// want: every account's ids are already sorted on disk, so the owner set could
// be built by merging sorted runs instead of hashing strings.
func BenchmarkOwnedGamesUnionUint32(b *testing.B) {
	data := newOwnedGamesBenchData()
	lists := make([][]uint32, 0, len(data.accounts))
	for _, entry := range data.accounts {
		lists = append(lists, entry.AppIDs)
	}
	b.Run("sortedMerge", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			merged := make([]uint32, 0, benchAppIDsPerAcct*benchVaultAccounts)
			for _, list := range lists {
				merged = append(merged, list...)
			}
			slices.Sort(merged)
			merged = slices.Compact(merged)
			if len(merged) != data.union {
				b.Fatalf("union = %d, want %d", len(merged), data.union)
			}
		}
	})
	b.Run("mapSet", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			set := make(map[uint32]struct{}, benchAppIDsPerAcct*2)
			for _, list := range lists {
				for _, id := range list {
					set[id] = struct{}{}
				}
			}
			if len(set) != data.union {
				b.Fatalf("union = %d, want %d", len(set), data.union)
			}
		}
	})
}

// BenchmarkOwnedGamesMembership is the installed-games pass: one lookup per
// installed app against the owner set, as a string map today and as a binary
// search over the sorted union for comparison.
func BenchmarkOwnedGamesMembership(b *testing.B) {
	data := newOwnedGamesBenchData()
	owners := benchOwners(data)
	installedStr := make([]string, 0, len(data.installed))
	installedU32 := make([]uint32, 0, len(data.installed))
	for _, game := range data.installed {
		installedStr = append(installedStr, game.AppID)
		id, _ := strconv.ParseUint(game.AppID, 10, 32)
		installedU32 = append(installedU32, uint32(id))
	}
	union := make([]uint32, 0, len(owners))
	for id := range owners {
		v, _ := strconv.ParseUint(id, 10, 32)
		union = append(union, uint32(v))
	}
	slices.Sort(union)

	b.Run("stringMap", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			hits := 0
			for _, id := range installedStr {
				if _, ok := owners[id]; ok {
					hits++
				}
			}
			if hits == 0 {
				b.Fatal("no hits")
			}
		}
	})
	b.Run("binarySearch", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			hits := 0
			for _, id := range installedU32 {
				if _, ok := slices.BinarySearch(union, id); ok {
					hits++
				}
			}
			if hits == 0 {
				b.Fatal("no hits")
			}
		}
	})
}
