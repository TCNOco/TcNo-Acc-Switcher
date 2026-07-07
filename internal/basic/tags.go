package basic

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/stats"
)

const maxTagNameLen = 64

// AccountTagDTO is a resolved tag on an account row (matches TagDefinitionDTO shape).
type AccountTagDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

// TagDefinitionDTO is one row in the global tag list for a platform.
type TagDefinitionDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

type tagFileEntry struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

func normalizeTagMaps(f *idsFile) {
	if f.Tags == nil {
		f.Tags = map[string]tagFileEntry{}
	}
	if f.AccountTags == nil {
		f.AccountTags = map[string][]string{}
	}
	if f.AccountTagExpiries == nil {
		f.AccountTagExpiries = map[string]map[string]string{}
	}
}

func pruneUnusedTagDefinitions(f *idsFile) {
	normalizeTagMaps(f)
	used := make(map[string]struct{})
	for uid, ids := range f.AccountTags {
		clean := ids[:0]
		seen := make(map[string]struct{}, len(ids))
		expiryMap := f.AccountTagExpiries[uid]
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			if _, ok := f.Tags[id]; !ok {
				continue
			}
			seen[id] = struct{}{}
			if _, seen := used[id]; !seen {
				used[id] = struct{}{}
			}
			clean = append(clean, id)
		}
		if len(clean) == 0 {
			delete(f.AccountTags, uid)
			delete(f.AccountTagExpiries, uid)
		} else {
			f.AccountTags[uid] = clean
			if len(expiryMap) > 0 {
				for id := range expiryMap {
					if !containsTagID(clean, id) {
						delete(expiryMap, id)
					}
				}
			}
			if len(expiryMap) == 0 {
				delete(f.AccountTagExpiries, uid)
			} else {
				f.AccountTagExpiries[uid] = expiryMap
			}
		}
	}
	for uid := range f.AccountTagExpiries {
		if len(f.AccountTags[uid]) == 0 {
			delete(f.AccountTagExpiries, uid)
		}
	}
	for id := range f.Tags {
		if _, ok := used[id]; !ok {
			delete(f.Tags, id)
		}
	}
}

func newTagID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func randomSaturatedColorHex() (string, error) {
	var rb [3]byte
	if _, err := rand.Read(rb[:]); err != nil {
		return "", err
	}
	h := float64(uint16(rb[0])|uint16(rb[1])<<8) / 65535.0 * 360.0
	s := 0.75 + (float64(rb[2])/255.0)*0.25 // 0.75–1.0
	l := 0.42 + (float64(rb[2])/255.0)*0.10 // 0.42–0.52
	r, g, b := hslToRGB(h, s, l)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b), nil
}

func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	s = clamp01(s)
	l = clamp01(l)
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var rp, gp, bp float64
	switch {
	case h < 60:
		rp, gp, bp = c, x, 0
	case h < 120:
		rp, gp, bp = x, c, 0
	case h < 180:
		rp, gp, bp = 0, c, x
	case h < 240:
		rp, gp, bp = 0, x, c
	case h < 300:
		rp, gp, bp = x, 0, c
	default:
		rp, gp, bp = c, 0, x
	}
	return uint8(math.Round((rp + m) * 255)), uint8(math.Round((gp + m) * 255)), uint8(math.Round((bp + m) * 255))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func resolveTagsForAccount(f idsFile, uniqueID string) []AccountTagDTO {
	normalizeTagMaps(&f)
	uniqueID = strings.TrimSpace(uniqueID)
	ids := f.AccountTags[uniqueID]
	if len(ids) == 0 {
		return nil
	}
	out := make([]AccountTagDTO, 0, len(ids))
	for _, tid := range ids {
		tid = strings.TrimSpace(tid)
		if tid == "" {
			continue
		}
		def, ok := f.Tags[tid]
		if !ok {
			continue
		}
		out = append(out, AccountTagDTO{
			ID:        tid,
			Name:      strings.TrimSpace(def.Name),
			Color:     strings.TrimSpace(def.Color),
			ExpiresAt: effectiveAccountTagExpiry(f, uniqueID, tid),
		})
	}
	return out
}

func listTagDefinitionsSorted(f idsFile) []TagDefinitionDTO {
	normalizeTagMaps(&f)
	type pair struct {
		id string
		t  tagFileEntry
	}
	var pairs []pair
	for id, t := range f.Tags {
		pairs = append(pairs, pair{id: id, t: t})
	}
	sort.Slice(pairs, func(i, j int) bool {
		ni := strings.ToLower(strings.TrimSpace(pairs[i].t.Name))
		nj := strings.ToLower(strings.TrimSpace(pairs[j].t.Name))
		if ni != nj {
			return ni < nj
		}
		return pairs[i].id < pairs[j].id
	})
	out := make([]TagDefinitionDTO, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, TagDefinitionDTO{
			ID:        p.id,
			Name:      strings.TrimSpace(p.t.Name),
			Color:     strings.TrimSpace(p.t.Color),
			ExpiresAt: strings.TrimSpace(p.t.ExpiresAt),
		})
	}
	return out
}

// ForgetAccountTagAssignments removes accountTags[uniqueID] from ids.json.
func ForgetAccountTagAssignments(platformKey, uniqueID string) error {
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	if platformKey == "" || uniqueID == "" {
		return nil
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return err
	}
	normalizeTagMaps(&f)
	delete(f.AccountTags, uniqueID)
	delete(f.AccountTagExpiries, uniqueID)
	pruneUnusedTagDefinitions(&f)
	if err := writeIdsFile(platformKey, f); err != nil {
		return err
	}
	_ = stats.SyncPlatformTagCounts(platformKey, len(f.Tags), countTaggedAccounts(f))
	return nil
}

func tagNameOK(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("empty tag name")
	}
	if len(name) > maxTagNameLen {
		name = name[:maxTagNameLen]
	}
	return name, nil
}

func containsTagID(slice []string, id string) bool {
	for _, x := range slice {
		if strings.EqualFold(strings.TrimSpace(x), id) {
			return true
		}
	}
	return false
}

func countTaggedAccounts(f idsFile) int {
	normalizeTagMaps(&f)
	return len(f.AccountTags)
}

func normalizeExpiryString(expiresAt string) (string, error) {
	expiresAt = strings.TrimSpace(expiresAt)
	if expiresAt == "" {
		return "", nil
	}
	ts, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return "", err
	}
	return ts.UTC().Format(time.RFC3339), nil
}

func isExpiredAt(expiresAt string, now time.Time) (bool, bool) {
	expiresAt = strings.TrimSpace(expiresAt)
	if expiresAt == "" {
		return false, false
	}
	ts, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return false, true
	}
	return !ts.UTC().After(now.UTC()), false
}

func earlierExpiry(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	at, aerr := time.Parse(time.RFC3339, a)
	bt, berr := time.Parse(time.RFC3339, b)
	if aerr != nil {
		return b
	}
	if berr != nil {
		return a
	}
	if bt.Before(at) {
		return bt.UTC().Format(time.RFC3339)
	}
	return at.UTC().Format(time.RFC3339)
}

func effectiveAccountTagExpiry(f idsFile, uniqueID, tagID string) string {
	defExpiry := ""
	if def, ok := f.Tags[tagID]; ok {
		defExpiry = strings.TrimSpace(def.ExpiresAt)
	}
	accountExpiry := ""
	if f.AccountTagExpiries != nil {
		accountExpiry = strings.TrimSpace(f.AccountTagExpiries[uniqueID][tagID])
	}
	return earlierExpiry(defExpiry, accountExpiry)
}

func clearAccountTagExpiry(f *idsFile, uniqueID, tagID string) {
	if f.AccountTagExpiries == nil {
		return
	}
	expiryMap := f.AccountTagExpiries[uniqueID]
	if len(expiryMap) == 0 {
		delete(f.AccountTagExpiries, uniqueID)
		return
	}
	delete(expiryMap, tagID)
	if len(expiryMap) == 0 {
		delete(f.AccountTagExpiries, uniqueID)
		return
	}
	f.AccountTagExpiries[uniqueID] = expiryMap
}

func pruneExpiredTagsInFile(f *idsFile, now time.Time) bool {
	normalizeTagMaps(f)
	changed := false
	expiredEverywhere := make(map[string]struct{})
	for id, def := range f.Tags {
		def.Name = strings.TrimSpace(def.Name)
		def.Color = strings.TrimSpace(def.Color)
		if expired, invalid := isExpiredAt(def.ExpiresAt, now); invalid {
			def.ExpiresAt = ""
			f.Tags[id] = def
			changed = true
			continue
		} else if expired {
			expiredEverywhere[id] = struct{}{}
		}
		f.Tags[id] = def
	}
	for uid, ids := range f.AccountTags {
		expiryMap := f.AccountTagExpiries[uid]
		next := ids[:0]
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				changed = true
				continue
			}
			if _, ok := f.Tags[id]; !ok {
				changed = true
				continue
			}
			if _, ok := expiredEverywhere[id]; ok {
				changed = true
				clearAccountTagExpiry(f, uid, id)
				continue
			}
			expiresAt := ""
			if expiryMap != nil {
				expiresAt = expiryMap[id]
			}
			if expired, invalid := isExpiredAt(expiresAt, now); invalid {
				clearAccountTagExpiry(f, uid, id)
				changed = true
			} else if expired {
				clearAccountTagExpiry(f, uid, id)
				changed = true
				continue
			}
			next = append(next, id)
		}
		if len(next) != len(ids) {
			changed = true
		}
		if len(next) == 0 {
			delete(f.AccountTags, uid)
			delete(f.AccountTagExpiries, uid)
			continue
		}
		f.AccountTags[uid] = next
	}
	beforeTags := len(f.Tags)
	beforeAccounts := len(f.AccountTags)
	beforeExpiries := len(f.AccountTagExpiries)
	pruneUnusedTagDefinitions(f)
	if len(f.Tags) != beforeTags || len(f.AccountTags) != beforeAccounts || len(f.AccountTagExpiries) != beforeExpiries {
		changed = true
	}
	return changed
}

func setTagExpiryOnFile(f *idsFile, uniqueID, tagID, scope, expiresAt string) error {
	normalizeTagMaps(f)
	tagID = strings.TrimSpace(tagID)
	uniqueID = strings.TrimSpace(uniqueID)
	scope = strings.TrimSpace(scope)
	normalized, err := normalizeExpiryString(expiresAt)
	if err != nil {
		return err
	}
	if _, ok := f.Tags[tagID]; !ok {
		return fmt.Errorf("unknown tag id")
	}
	switch scope {
	case "account":
		if uniqueID == "" {
			return fmt.Errorf("invalid tag parameters")
		}
		if !containsTagID(f.AccountTags[uniqueID], tagID) {
			return fmt.Errorf("tag not assigned to account")
		}
		if normalized == "" {
			clearAccountTagExpiry(f, uniqueID, tagID)
			return nil
		}
		expiryMap := f.AccountTagExpiries[uniqueID]
		if expiryMap == nil {
			expiryMap = map[string]string{}
		}
		expiryMap[tagID] = normalized
		f.AccountTagExpiries[uniqueID] = expiryMap
		return nil
	case "all":
		def := f.Tags[tagID]
		def.ExpiresAt = normalized
		f.Tags[tagID] = def
		return nil
	default:
		return fmt.Errorf("invalid tag expiry scope")
	}
}

func nextCS2DropReset(now time.Time) time.Time {
	now = now.UTC()
	reset := time.Date(now.Year(), now.Month(), now.Day(), 1, 0, 0, 0, time.UTC)
	daysUntil := (int(time.Wednesday) - int(reset.Weekday()) + 7) % 7
	reset = reset.AddDate(0, 0, daysUntil)
	if !reset.After(now) {
		reset = reset.AddDate(0, 0, 7)
	}
	return reset
}

// BuildAccountTagMap returns resolved tag DTOs per unique account id (platform cache key).
func BuildAccountTagMap(platformKey string) (map[string][]AccountTagDTO, error) {
	f, err := readIdsFile(platformKey)
	if err != nil {
		return nil, err
	}
	normalizeTagMaps(&f)
	m := make(map[string][]AccountTagDTO)
	for uid := range f.AccountTags {
		t := resolveTagsForAccount(f, uid)
		if len(t) > 0 {
			m[uid] = t
		}
	}
	return m, nil
}
