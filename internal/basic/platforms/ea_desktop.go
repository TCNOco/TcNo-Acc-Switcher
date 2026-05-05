package platforms

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func EAImageSource(cacheDataPattern, userIniPattern string) (ProfileImageSource, error) {
	userIDs := eaUserIDsByRecency(userIniPattern)
	matches, err := filepath.Glob(strings.TrimSpace(cacheDataPattern))
	if err != nil || len(matches) == 0 {
		return ProfileImageSource{}, err
	}
	sortByModDesc(matches)
	for _, f := range matches {
		st, err := os.Stat(f)
		if err != nil || st.IsDir() {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil || len(data) == 0 {
			continue
		}
		url := parseEAAvatarURLForUserIDs(data, userIDs)
		if strings.TrimSpace(url) != "" {
			return ProfileImageSource{RemoteURL: url}, nil
		}
	}
	return ProfileImageSource{}, nil
}

func EASuggestedName(dataPattern, userIniPattern string) (string, error) {
	userIDs := eaUserIDsByRecency(userIniPattern)
	files, err := filepath.Glob(strings.TrimSpace(dataPattern))
	if err != nil || len(files) == 0 {
		return "", err
	}
	sortByModDesc(files)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil || len(data) == 0 {
			continue
		}
		if n := parseEANameForUserIDs(data, userIDs); n != "" {
			return n, nil
		}
	}
	return "", nil
}

func parseEAAvatarURLForUserIDs(data []byte, userIDs []string) string {
	// When we know candidate user IDs, do NOT fall back to "first me" match.
	// Otherwise we can pick a different user from another cached response.
	if len(userIDs) == 0 {
		return parseEAAvatarURLForUserID(data, "")
	}
	for _, uid := range userIDs {
		url := parseEAAvatarURLForUserID(data, uid)
		if strings.TrimSpace(url) != "" {
			return url
		}
	}
	return ""
}

func parseEAAvatarURLForUserID(data []byte, userID string) string {
	uid := strings.TrimSpace(userID)
	if uid == "" {
		return firstLargeURL(data)
	}

	idNeedle := []byte(`"id":"` + uid + `"`)
	if len(idNeedle) == 0 {
		return ""
	}

	blockMarker := []byte(`{"data":{`)
	off := 0
	for {
		idPosRel := bytes.Index(data[off:], idNeedle)
		if idPosRel < 0 {
			return ""
		}
		idPos := off + idPosRel

		// Treat a "block" as from nearest preceding {"data":{ to the next one.
		blockStart := bytes.LastIndex(data[:idPos], blockMarker)
		if blockStart < 0 {
			blockStart = 0
		}
		blockEnd := len(data)
		if next := bytes.Index(data[idPos:], blockMarker); next > 0 {
			blockEnd = idPos + next
		}
		if blockEnd > blockStart {
			if u := firstLargeURL(data[blockStart:blockEnd]); u != "" {
				return u
			}
		}

		off = idPos + 1
	}
}

func firstLargeURL(data []byte) string {
	search := data
	for {
		largeIdx := bytes.Index(search, []byte(`"large"`))
		if largeIdx < 0 {
			return ""
		}
		afterLarge := search[largeIdx:]
		pathIdx := bytes.Index(afterLarge, []byte(`"path":"`))
		if pathIdx < 0 {
			search = afterLarge[len(`"large"`):]
			continue
		}
		rest := afterLarge[pathIdx+len(`"path":"`):]
		end := bytes.IndexByte(rest, '"')
		if end <= 0 {
			search = afterLarge[len(`"large"`):]
			continue
		}
		u := strings.TrimSpace(string(rest[:end]))
		u = strings.ReplaceAll(u, `\/`, `/`)
		if strings.HasPrefix(strings.ToLower(u), "https://") || strings.HasPrefix(strings.ToLower(u), "http://") {
			return u
		}
		search = afterLarge[len(`"large"`):]
	}
}

func eaUserIDsByRecency(userIniPattern string) []string {
	files, err := filepath.Glob(strings.TrimSpace(userIniPattern))
	if err != nil || len(files) == 0 {
		return nil
	}
	sortByModDesc(files)
	seen := map[string]struct{}{}
	out := make([]string, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil || len(data) == 0 {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
			if !strings.HasPrefix(strings.ToLower(line), "user.userid=") {
				continue
			}
			id := strings.TrimSpace(line[len("user.userid="):])
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
			break
		}
	}
	return out
}

func parseEANameForUserIDs(data []byte, userIDs []string) string {
	if len(userIDs) == 0 {
		return parseEANameForUserID(data, "")
	}
	for _, uid := range userIDs {
		if n := parseEANameForUserID(data, uid); n != "" {
			return n
		}
	}
	return ""
}

func parseEANameForUserID(data []byte, userID string) string {
	const marker = `{"data":{"me":`
	searchFrom := data
	if strings.TrimSpace(userID) != "" {
		userMarker := []byte(`{"data":{"me":{"id":"` + strings.TrimSpace(userID) + `"`)
		start := bytes.Index(data, userMarker)
		if start < 0 {
			return ""
		}
		searchFrom = data[start:]
	}
	start := bytes.Index(searchFrom, []byte(marker))
	if start < 0 {
		return ""
	}
	chunk := searchFrom[start:]
	// Prefer the same payload section used for avatar extraction:
	// me.player.* + avatar.large.path.
	largeIdx := bytes.Index(chunk, []byte(`"large"`))
	if largeIdx > 0 {
		if n := parseEANameFromPlayerWindow(chunk[:largeIdx]); n != "" {
			return n
		}
	}
	// Fallback when avatar info is not present in this cache record.
	if n := parseEANameFromPlayerWindow(chunk); n != "" {
		return n
	}
	return ""
}

func parseEANameFromPlayerWindow(window []byte) string {
	// Stay close to me.player to avoid personas[].displayName from other sections.
	playerIdx := bytes.Index(window, []byte(`"player":{`))
	if playerIdx >= 0 {
		window = window[playerIdx:]
	}
	for _, key := range []string{`"displayName":"`, `"uniqueName":"`, `"nickname":"`} {
		if v := parseEAStringField(window, key); v != "" {
			return v
		}
	}
	return ""
}

func parseEAStringField(data []byte, key string) string {
	i := bytes.Index(data, []byte(key))
	if i < 0 {
		return ""
	}
	rest := data[i+len(key):]
	j := bytes.IndexByte(rest, '"')
	if j <= 0 {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(string(rest[:j]), `\/`, `/`))
}

func sortByModDesc(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		ist, ierr := os.Stat(paths[i])
		jst, jerr := os.Stat(paths[j])
		if ierr != nil || jerr != nil {
			return paths[i] > paths[j]
		}
		return ist.ModTime().After(jst.ModTime())
	})
}
