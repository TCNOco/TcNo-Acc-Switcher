package winutil

import "strings"

// ShortcutAppUserModelID builds a stable Windows shell identity for a .lnk file.
// Unique IDs prevent Start Menu / pinned tiles from sharing one icon when shortcuts target the same exe.
func ShortcutAppUserModelID(parts ...string) string {
	var b strings.Builder
	b.WriteString("TcNo.AccountSwitcher")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		b.WriteByte('.')
		for _, r := range p {
			switch {
			case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
				b.WriteRune(r)
			case r == '_', r == '-':
				b.WriteRune(r)
			default:
				b.WriteRune('_')
			}
		}
	}
	out := b.String()
	if len(out) > 128 {
		out = out[:128]
	}
	return out
}
