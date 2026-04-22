package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// Kind describes the primary CLI intent after parsing.
type Kind int

const (
	KindNone Kind = iota
	KindSwapSteam
	KindSwapBasic
	KindLogout
	KindHelp
	KindOpenPage
)

// Parsed is the normalized CLI result.
type Parsed struct {
	Kind           Kind
	Verbose        bool
	Help           bool
	SteamID64      string
	PersonaState   int // steam swap; -1 means default / unchanged
	PlatformKey    string // canonical platform name from Platforms.json
	UniqueID       string // basic platforms
	LogoutPlatform string // optional filter for logout
	LogoutAccount  string // optional id for logout (reserved)
	OpenPage       string // platform name for GUI route
}

const steamPlatformName = "Steam"

// NormalizeProtocolArg strips tcno:// and returns the payload path (e.g. "s:765...").
func NormalizeProtocolArg(s string) string {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "tcno://") {
		s = s[len("tcno://"):]
	} else if strings.HasPrefix(low, "tcno:") && len(s) > 5 && s[5] == '/' {
		s = s[5:]
		if strings.HasPrefix(s, "//") {
			s = s[2:]
		}
	}
	return strings.TrimSpace(s)
}

// Parse inspects argv (typically os.Args[1:]) with optional platform index from LoadPlatformIndex.
func Parse(argv []string, idx *PlatformIndex) (Parsed, error) {
	var p Parsed
	p.PersonaState = -1

	if len(argv) == 0 {
		return p, nil
	}

	expanded := make([]string, 0, len(argv))
	for _, a := range argv {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if strings.Contains(a, "://") || strings.HasPrefix(strings.ToLower(a), "tcno:") {
			a = NormalizeProtocolArg(a)
		}
		expanded = append(expanded, a)
	}

	for _, a := range expanded {
		al := strings.ToLower(a)
		switch al {
		case "-h", "--help", "help", "/?":
			p.Help = true
			p.Kind = KindHelp
			continue
		case "-v", "--verbose", "verbose":
			p.Verbose = true
			continue
		}

		if isLogoutToken(a) {
			lp, err := logoutParsed(a)
			if err != nil {
				return Parsed{}, err
			}
			if err := mergePrimary(&p, lp); err != nil {
				return Parsed{}, err
			}
			continue
		}

		if strings.HasPrefix(a, "+") {
			pp, err := parsePlusToken(a, idx)
			if err != nil {
				return Parsed{}, err
			}
			if err := mergePrimary(&p, pp); err != nil {
				return Parsed{}, err
			}
			continue
		}

		if idx != nil {
			if canon, ok := idx.Names[strings.ToLower(a)]; ok {
				pp := Parsed{Kind: KindOpenPage, OpenPage: canon}
				if err := mergePrimary(&p, pp); err != nil {
					return Parsed{}, err
				}
				continue
			}
		}
		return Parsed{}, fmt.Errorf("unknown argument: %s", a)
	}

	if p.Help && p.Kind != KindHelp && p.Kind == KindNone {
		p.Kind = KindHelp
	}

	return p, nil
}

func isLogoutToken(a string) bool {
	s := strings.TrimSpace(a)
	lo := strings.ToLower(s)
	if lo == "logout" {
		return true
	}
	return strings.HasPrefix(lo, "logout:") || strings.HasPrefix(lo, "logout ")
}

func logoutParsed(a string) (Parsed, error) {
	p := Parsed{Kind: KindLogout}
	s := strings.TrimSpace(a)
	// strip "logout" prefix (any case)
	lo := strings.ToLower(s)
	if !strings.HasPrefix(lo, "logout") {
		return Parsed{}, fmt.Errorf("not a logout token")
	}
	s = strings.TrimSpace(s[len("logout"):])
	rest := s
	if strings.HasPrefix(rest, ":") {
		rest = rest[1:]
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return p, nil
	}
	parts := strings.Split(rest, ":")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) >= 1 && parts[0] != "" {
		p.LogoutPlatform = parts[0]
	}
	if len(parts) >= 2 && parts[1] != "" {
		p.LogoutAccount = parts[1]
	}
	return p, nil
}

func mergePrimary(dst *Parsed, src Parsed) error {
	if src.Kind == KindNone {
		return nil
	}
	if dst.Kind != KindNone && dst.Kind != src.Kind {
		return fmt.Errorf("conflicting commands (already %v, got %v)", dst.Kind, src.Kind)
	}
	dst.Kind = src.Kind
	dst.SteamID64 = src.SteamID64
	dst.PersonaState = src.PersonaState
	dst.PlatformKey = src.PlatformKey
	dst.UniqueID = src.UniqueID
	dst.OpenPage = src.OpenPage
	dst.LogoutPlatform = src.LogoutPlatform
	dst.LogoutAccount = src.LogoutAccount
	return nil
}

func parsePlusToken(a string, idx *PlatformIndex) (Parsed, error) {
	if !strings.HasPrefix(a, "+") {
		return Parsed{}, fmt.Errorf("invalid token")
	}
	body := a[1:]
	idxColon := strings.Index(body, ":")
	if idxColon < 0 {
		return Parsed{}, fmt.Errorf("expected +prefix:value in %q", a)
	}
	prefix := strings.ToLower(strings.TrimSpace(body[:idxColon]))
	val := strings.TrimSpace(body[idxColon+1:])
	if prefix == "" || val == "" {
		return Parsed{}, fmt.Errorf("empty +prefix:value in %q", a)
	}

	if prefix == "s" {
		return parseSteamSwap(val)
	}

	if idx == nil {
		return Parsed{}, fmt.Errorf("unknown platform prefix %q (platforms file not loaded)", prefix)
	}

	platformName, ok := idx.FirstIdentifier[prefix]
	if !ok {
		return Parsed{}, fmt.Errorf("unknown platform prefix %q", prefix)
	}

	if strings.EqualFold(platformName, steamPlatformName) {
		return parseSteamSwap(val)
	}

	return Parsed{
		Kind:        KindSwapBasic,
		PlatformKey: platformName,
		UniqueID:    val,
	}, nil
}

func parseSteamSwap(val string) (Parsed, error) {
	parts := strings.Split(val, ":")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) == 0 || parts[0] == "" {
		return Parsed{}, fmt.Errorf("empty steam id in +s:")
	}
	id := parts[0]
	state := -1
	if len(parts) >= 2 && parts[1] != "" {
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			return Parsed{}, fmt.Errorf("invalid persona state: %w", err)
		}
		state = n
	}
	return Parsed{
		Kind:         KindSwapSteam,
		PlatformKey:  steamPlatformName,
		SteamID64:    id,
		PersonaState: state,
	}, nil
}

// NeedsHeadlessMutex returns true when this process might run swap/logout without the GUI first.
func (p Parsed) NeedsHeadlessMutex() bool {
	switch p.Kind {
	case KindSwapSteam, KindSwapBasic, KindLogout:
		return true
	default:
		return false
	}
}

// RouteJSONForOpenPage returns a JSON string for the Wails "navigate" event payload.
func (p Parsed) RouteJSONForOpenPage() string {
	if p.Kind != KindOpenPage || strings.TrimSpace(p.OpenPage) == "" {
		return ""
	}
	name := strings.ReplaceAll(p.OpenPage, `\`, `\\`)
	name = strings.ReplaceAll(name, `"`, `\"`)
	return fmt.Sprintf(`{"page":"platform","platformName":"%s"}`, name)
}
