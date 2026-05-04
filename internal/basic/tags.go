package basic

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

const maxTagNameLen = 64

// AccountTagDTO is a resolved tag on an account row (matches TagDefinitionDTO shape).
type AccountTagDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// TagDefinitionDTO is one row in the global tag list for a platform.
type TagDefinitionDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type tagFileEntry struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func normalizeTagMaps(f *idsFile) {
	if f.Tags == nil {
		f.Tags = map[string]tagFileEntry{}
	}
	if f.AccountTags == nil {
		f.AccountTags = map[string][]string{}
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
	return uint8(math.Round((rp+m)*255)), uint8(math.Round((gp+m)*255)), uint8(math.Round((bp+m)*255))
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
	ids := f.AccountTags[strings.TrimSpace(uniqueID)]
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
			ID:    tid,
			Name:  strings.TrimSpace(def.Name),
			Color: strings.TrimSpace(def.Color),
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
			ID:    p.id,
			Name:  strings.TrimSpace(p.t.Name),
			Color: strings.TrimSpace(p.t.Color),
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
	return writeIdsFile(platformKey, f)
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
