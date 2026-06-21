package logsanitize

import (
	"fmt"
	"sort"
	"strings"
)

type secretReplacement struct {
	secret      string
	replacement string
}

// Redact replaces known account identifiers in text with accountN aliases (best-effort).
func Redact(text string) string {
	reps := collectReplacements()
	if len(reps) == 0 {
		return text
	}
	out := text
	for _, r := range reps {
		out = replaceCI(out, r.secret, r.replacement)
	}
	return out
}

func collectReplacements() []secretReplacement {
	accounts := collectAccountIdentifiers()
	if len(accounts) == 0 {
		return nil
	}
	var reps []secretReplacement
	seen := map[string]struct{}{}
	for i, ids := range accounts {
		base := fmt.Sprintf("account%d", i+1)
		for _, secret := range ids {
			secret = strings.TrimSpace(secret)
			if secret == "" {
				continue
			}
			key := strings.ToLower(secret)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			reps = append(reps, secretReplacement{
				secret:      secret,
				replacement: aliasForAccount(base, secret),
			})
		}
	}
	sort.Slice(reps, func(i, j int) bool {
		return len(reps[i].secret) > len(reps[j].secret)
	})
	return reps
}

func aliasForAccount(base, original string) string {
	i := len(original)
	for i > 0 {
		c := original[i-1]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			break
		}
		i--
	}
	return base + original[i:]
}

func replaceCI(s, old, new string) string {
	if old == "" || s == "" {
		return s
	}
	lower := strings.ToLower(s)
	oldLower := strings.ToLower(old)
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for {
		j := strings.Index(lower[i:], oldLower)
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		j += i
		b.WriteString(s[i:j])
		b.WriteString(new)
		i = j + len(old)
	}
	return b.String()
}
