package logsanitize

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"TcNo-Acc-Switcher/internal/logredact"
)

type secretReplacement struct {
	secret      string
	replacement string
}

// Redact replaces known account identifiers in text with accountN aliases (best-effort).
func Redact(text string) string {
	return redactWithAccounts(text, collectAccountIdentifiers())
}

func redactWithAccounts(text string, accounts [][]string) string {
	text = logredact.RedactText(text)
	reps := replacementsForAccounts(accounts)
	if len(reps) == 0 {
		return text
	}
	// Replace once: an account actually named "account1" must not rewrite an
	// alias inserted for a different account. Regex matching also preserves
	// byte offsets when Unicode case folding changes a character's length.
	patterns := make([]string, 0, len(reps))
	replacements := make(map[string]string, len(reps))
	for _, r := range reps {
		patterns = append(patterns, regexp.QuoteMeta(r.secret))
		replacements[strings.ToLower(r.secret)] = r.replacement
	}
	pattern, err := regexp.Compile("(?i)" + strings.Join(patterns, "|"))
	if err != nil {
		return "[diagnostic text omitted: account redaction failed]"
	}
	return pattern.ReplaceAllStringFunc(text, func(match string) string {
		if replacement, ok := replacements[strings.ToLower(match)]; ok {
			return replacement
		}
		for _, r := range reps {
			if strings.EqualFold(match, r.secret) {
				return r.replacement
			}
		}
		return "[account]"
	})
}

func collectReplacements() []secretReplacement {
	return replacementsForAccounts(collectAccountIdentifiers())
}

func replacementsForAccounts(accounts [][]string) []secretReplacement {
	accounts = mergeAccountGroups(accounts)
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

// mergeAccountGroups keeps usernames, SteamID64, and userdata IDs on the same
// alias even when an attempt initially knew only the target's ID.
func mergeAccountGroups(accounts [][]string) [][]string {
	parent := make([]int, len(accounts))
	for i := range parent {
		parent[i] = i
	}
	var root func(int) int
	root = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	owners := map[string]int{}
	for i, group := range accounts {
		for _, id := range group {
			key := strings.ToLower(strings.TrimSpace(id))
			if key == "" {
				continue
			}
			if previous, ok := owners[key]; ok {
				a, b := root(i), root(previous)
				if a < b {
					parent[b] = a
				} else {
					parent[a] = b
				}
			} else {
				owners[key] = i
			}
		}
	}
	merged := make([][]string, len(accounts))
	for i, group := range accounts {
		r := root(i)
		merged[r] = append(merged[r], group...)
	}
	var out [][]string
	for _, group := range merged {
		if len(group) > 0 {
			out = append(out, group)
		}
	}
	return out
}
