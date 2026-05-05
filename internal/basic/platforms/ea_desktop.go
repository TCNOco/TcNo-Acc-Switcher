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
	for _, uid := range userIDs {
		url := parseEAAvatarURLForUserID(data, uid)
		if url != "" {
			return url
		}
	}
	return parseEAAvatarURLForUserID(data, "")
}

func parseEAAvatarURLForUserID(data []byte, userID string) string {
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
	largeIdx := bytes.Index(chunk, []byte(`"large"`))
	if largeIdx < 0 {
		return ""
	}
	chunk = chunk[largeIdx:]
	pathIdx := bytes.Index(chunk, []byte(`"path":"`))
	if pathIdx < 0 {
		return ""
	}
	chunk = chunk[pathIdx+len(`"path":"`):]
	end := bytes.IndexByte(chunk, '"')
	if end <= 0 {
		return ""
	}
	u := strings.TrimSpace(string(chunk[:end]))
	u = strings.ReplaceAll(u, `\/`, `/`)
	if strings.HasPrefix(strings.ToLower(u), "https://") || strings.HasPrefix(strings.ToLower(u), "http://") {
		return u
	}
	return ""
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
	for _, uid := range userIDs {
		if n := parseEANameForUserID(data, uid); n != "" {
			return n
		}
	}
	return parseEANameForUserID(data, "")
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
	for _, key := range []string{`"nickname":"`, `"displayName":"`, `"uniqueName":"`} {
		i := bytes.Index(chunk, []byte(key))
		if i < 0 {
			continue
		}
		rest := chunk[i+len(key):]
		j := bytes.IndexByte(rest, '"')
		if j <= 0 {
			continue
		}
		v := strings.TrimSpace(strings.ReplaceAll(string(rest[:j]), `\/`, `/`))
		if v != "" {
			return v
		}
	}
	return ""
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
