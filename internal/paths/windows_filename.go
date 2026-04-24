package paths

import (
	"strings"
	"unicode"
)

// WindowsFileName normalizes arbitrary user text for a single Windows path segment
// (folder name, .lnk base name). Letters and numbers from any script are kept;
// Windows-forbidden characters, control/format characters, and most symbols (e.g. emoji)
// become underscores. Trailing spaces and dots are stripped. maxRunes > 0 truncates by rune count.
func WindowsFileName(name string, maxRunes int) string {
	s := strings.TrimSpace(name)
	s = strings.ToValidUTF8(s, "")
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 32:
			b.WriteRune('_')
		case r == '<' || r == '>' || r == ':' || r == '"' || r == '/' || r == '\\' || r == '|' || r == '?' || r == '*':
			b.WriteRune('_')
		case unicode.Is(unicode.Cf, r):
			continue
		case unicode.IsLetter(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Me, r):
			b.WriteRune(r)
		case unicode.IsNumber(r):
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '-' || r == '.' || r == '(' || r == ')' || r == ',' || r == '\'' || r == '&' || r == '+' || r == '[' || r == ']':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	trimCollapsed := func(s string) string {
		out := strings.TrimRight(s, " .")
		out = strings.TrimSpace(out)
		for strings.Contains(out, "__") {
			out = strings.ReplaceAll(out, "__", "_")
		}
		for strings.Contains(out, " _") || strings.Contains(out, "_ ") {
			out = strings.ReplaceAll(out, " _", "_")
			out = strings.ReplaceAll(out, "_ ", "_")
		}
		return strings.Trim(out, ".")
	}
	out := trimCollapsed(b.String())
	if out == "" || out == "." || out == ".." {
		return ""
	}
	if windowsReservedFileStem(out) {
		out = out + "_"
	}
	if maxRunes > 0 {
		rr := []rune(out)
		if len(rr) > maxRunes {
			out = trimCollapsed(string(rr[:maxRunes]))
		}
	}
	if out == "" || out == "." || out == ".." {
		return ""
	}
	if windowsReservedFileStem(out) {
		out = out + "_"
	}
	return out
}
