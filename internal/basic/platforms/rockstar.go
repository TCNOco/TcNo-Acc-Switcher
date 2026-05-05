package platforms

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

func RockstarImageSource(dataPattern string) (ProfileImageSource, error) {
	matches, err := filepath.Glob(strings.TrimSpace(dataPattern))
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
		if u := parseRockstarAvatarURL(data); u != "" {
			return ProfileImageSource{RemoteURL: u}, nil
		}
	}
	return ProfileImageSource{}, nil
}

func RockstarSuggestedName(dataPattern string) (string, error) {
	matches, err := filepath.Glob(strings.TrimSpace(dataPattern))
	if err != nil || len(matches) == 0 {
		return "", err
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
		if n := parseRockstarNickname(data); n != "" {
			return n, nil
		}
	}
	return "", nil
}

func parseRockstarAvatarURL(data []byte) string {
	const host = "prod-avatars.akamaized.net"
	start := bytes.Index(data, []byte(host))
	if start < 0 {
		return ""
	}
	i := start
	for i > 0 {
		c := data[i-1]
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '"' || c == '\'' || c == '<' || c == '>' || c == 0 {
			break
		}
		i--
	}
	u := strings.TrimSpace(string(data[i:]))
	for j, c := range u {
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '"' || c == '\'' || c == '<' || c == '>' || c == 0 {
			u = u[:j]
			break
		}
	}
	if k := strings.Index(strings.ToLower(u), "https://"); k >= 0 {
		u = u[k:]
	} else if k := strings.Index(strings.ToLower(u), "http://"); k >= 0 {
		u = u[k:]
	}
	u = strings.TrimSpace(strings.TrimSuffix(u, "."))
	if strings.HasPrefix(strings.ToLower(u), "https://") || strings.HasPrefix(strings.ToLower(u), "http://") {
		if strings.Contains(strings.ToLower(u), host) {
			return u
		}
	}
	return ""
}

func parseRockstarNickname(data []byte) string {
	const (
		accOpen   = "<RockstarAccount>"
		accClose  = "</RockstarAccount>"
		nickOpen  = "<Nickname>"
		nickClose = "</Nickname>"
	)
	searchFrom := data
	for {
		accStart := bytes.Index(searchFrom, []byte(accOpen))
		if accStart < 0 {
			return ""
		}
		searchFrom = searchFrom[accStart:]
		accEnd := bytes.Index(searchFrom, []byte(accClose))
		if accEnd < 0 {
			return ""
		}
		block := searchFrom[:accEnd+len(accClose)]
		nickStart := bytes.Index(block, []byte(nickOpen))
		if nickStart >= 0 {
			rest := block[nickStart+len(nickOpen):]
			nickEnd := bytes.Index(rest, []byte(nickClose))
			if nickEnd >= 0 {
				v := strings.TrimSpace(string(rest[:nickEnd]))
				v = strings.Map(func(r rune) rune {
					if r == 0 {
						return -1
					}
					return r
				}, v)
				if strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			}
		}
		searchFrom = searchFrom[accEnd+len(accClose):]
	}
}
