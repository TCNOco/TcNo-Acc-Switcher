package platforms

import (
	"bytes"
	"regexp"
	"strings"
	"unicode"
)

func RockstarImageSource(dataPattern string, fallbackPatterns ...string) (ProfileImageSource, error) {
	url, err := firstCacheValue(dataPattern, fallbackPatterns, parseRockstarAvatarURL)
	return ProfileImageSource{RemoteURL: url}, err
}

func RockstarSuggestedName(dataPattern string, fallbackPatterns ...string) (string, error) {
	return firstCacheValue(dataPattern, fallbackPatterns, parseRockstarNickname)
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
	// Cache blobs sometimes store XML-like text with interleaved NUL bytes.
	// Normalize first so tag matching works reliably.
	data = bytes.ReplaceAll(data, []byte{0}, nil)
	asText := string(data)
	// Prefer extracting from a complete RockstarAccount XML block to avoid
	// false-positive tag-like byte sequences in binary cache sections.
	re := regexp.MustCompile(`(?is)<RockstarAccount>.*?<Nickname>([^<]{1,64})</Nickname>.*?</RockstarAccount>`)
	matches := re.FindAllStringSubmatch(asText, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		if v := cleanCandidateName(m[1]); isLikelyNickname(v) {
			return v
		}
	}
	const (
		accOpen   = "<rockstaraccount>"
		accClose  = "</rockstaraccount>"
		nickOpen  = "<nickname>"
		nickClose = "</nickname>"
	)
	lower := bytes.ToLower(data)
	searchFrom := data
	searchFromLower := lower
	for {
		accStart := bytes.Index(searchFromLower, []byte(accOpen))
		if accStart < 0 {
			break
		}
		searchFrom = searchFrom[accStart:]
		searchFromLower = searchFromLower[accStart:]
		accEnd := bytes.Index(searchFromLower, []byte(accClose))
		if accEnd < 0 {
			break
		}
		block := searchFrom[:accEnd+len(accClose)]
		blockLower := searchFromLower[:accEnd+len(accClose)]
		nickStart := bytes.Index(blockLower, []byte(nickOpen))
		if nickStart >= 0 {
			rest := block[nickStart+len(nickOpen):]
			restLower := blockLower[nickStart+len(nickOpen):]
			nickEnd := bytes.Index(restLower, []byte(nickClose))
			if nickEnd >= 0 {
				if v := cleanCandidateName(string(rest[:nickEnd])); isLikelyNickname(v) {
					return v
				}
			}
		}
		searchFrom = searchFrom[accEnd+len(accClose):]
		searchFromLower = searchFromLower[accEnd+len(accClose):]
	}
	// Fallback: some cache records may contain <Nickname> outside a full <RockstarAccount> block.
	if nickStart := bytes.Index(lower, []byte(nickOpen)); nickStart >= 0 {
		rest := data[nickStart+len(nickOpen):]
		restLower := lower[nickStart+len(nickOpen):]
		if nickEnd := bytes.Index(restLower, []byte(nickClose)); nickEnd >= 0 {
			v := cleanCandidateName(string(rest[:nickEnd]))
			if isLikelyNickname(v) {
				return v
			}
		}
	}
	return ""
}

func cleanCandidateName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		if r == 0 {
			return -1
		}
		// Drop non-newline control chars often present in cache blobs.
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

func isLikelyNickname(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 64 {
		return false
	}
	hasLetterOrDigit := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			hasLetterOrDigit = true
		case r == ' ' || r == '_' || r == '-' || r == '.':
			// allowed separators
		default:
			return false
		}
	}
	return hasLetterOrDigit
}
