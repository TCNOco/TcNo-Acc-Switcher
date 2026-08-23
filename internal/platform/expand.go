package platform

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// PathTokenContext supplies values for %Platform_Folder%, %UniqueId%, %FileName%, %LARGEST%.
// For %LARGEST%.ext patterns, LargestPath is the matching filename stem (no extension).
type PathTokenContext struct {
	PlatformFolder string
	UniqueID       string
	FileName       string
	LargestPath    string
}

// pathToken is one %Name% placeholder a catalog path may use and the value this
// OS gives it. A token with no value here has none on this OS at all.
type pathToken struct {
	name  string
	value string
}

// pathTokens resolves the placeholder table for the running OS.
//
// Most entries are Windows shell folders and come back empty elsewhere, which is
// the point: a Linux build must not silently treat "%ProgramFiles(x86)%\Steam"
// as a relative directory. The home-anchored ones resolve everywhere, so a
// catalog can write %UserProfile% and mean it on any platform.
func pathTokens() []pathToken {
	// USERPROFILE first, and not only on Windows: it is what os.UserHomeDir reads
	// there anyway, and elsewhere it is the one handle a test has to move the
	// home-anchored tokens somewhere disposable. Other code resolves these bases
	// the same way, and the two have to agree - a cache path that expands against
	// one home and is safety-checked against another passes a check that was
	// meant to stop it deleting the real Desktop.
	home := strings.TrimSpace(os.Getenv("USERPROFILE"))
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}
	sub := func(name string) string {
		if home == "" {
			return ""
		}
		return filepath.Join(home, name)
	}
	return []pathToken{
		{"%ProgramFiles(x86)%", os.Getenv("ProgramFiles(x86)")},
		{"%ProgramFiles%", os.Getenv("ProgramFiles")},
		{"%LocalAppData%", os.Getenv("LocalAppData")},
		{"%AppData%", os.Getenv("AppData")},
		{"%ProgramData%", os.Getenv("ProgramData")},
		{"%StartMenuAppData%", filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`)},
		{"%StartMenuProgramData%", filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs`)},
		{"%UserProfile%", home},
		{"%USERPROFILE%", home},
		{"%Desktop%", sub("Desktop")},
		{"%Documents%", sub("Documents")},
		{"%Music%", sub("Music")},
		{"%Pictures%", sub("Pictures")},
		{"%Videos%", sub("Videos")},
	}
}

// ExpandWindowsPath resolves the %Name% placeholders a catalog path may carry.
//
// A path naming a placeholder this OS has no value for comes back empty rather
// than with the literal token left in it. Callers already read "" as "no path";
// a leftover "%ProgramFiles(x86)%\Steam\steam.exe" instead looks like a
// relative directory, and that is how a Linux build ends up reporting it looked
// for Steam somewhere no filesystem could ever have it.
//
// Separators are rewritten off Windows, because the catalogs are written with
// backslashes and nothing else on a Unix filesystem treats one as a separator.
// %Platform_Folder% and the other context tokens are left alone for
// [ExpandPathTokens] to fill in.
func ExpandWindowsPath(s string) string {
	if s == "" {
		return ""
	}
	s = strings.TrimSpace(s)
	out := s
	for _, tok := range pathTokens() {
		if !strings.Contains(out, tok.name) {
			continue
		}
		if tok.value == "" {
			return ""
		}
		out = strings.ReplaceAll(out, tok.name, tok.value)
	}
	if lt := strings.ToLower(strings.TrimSpace(out)); strings.HasPrefix(lt, "http://") || strings.HasPrefix(lt, "https://") {
		return out
	}
	if runtime.GOOS != "windows" {
		out = strings.ReplaceAll(out, `\`, "/")
	}
	return filepath.Clean(out)
}

// ExpandPathTokens applies standard env expansion then context tokens (order: env first, then ctx).
func ExpandPathTokens(s string, ctx PathTokenContext) string {
	s = ExpandWindowsPath(s)
	if ctx.PlatformFolder != "" {
		s = strings.ReplaceAll(s, "%Platform_Folder%", ctx.PlatformFolder)
	}
	if ctx.UniqueID != "" {
		s = strings.ReplaceAll(s, "%UniqueId%", ctx.UniqueID)
	}
	if ctx.FileName != "" {
		s = strings.ReplaceAll(s, "%FileName%", ctx.FileName)
	}
	if ctx.LargestPath != "" {
		s = strings.ReplaceAll(s, "%LARGEST%", ctx.LargestPath)
	}
	return s
}

var (
	emailRegex       = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	winFilepathRegex = regexp.MustCompile(`(?:[a-zA-Z]:\\|\\\\)[^:*?"<>|\r\n]+`)
)

const (
	RegexSentinelEmail   = "EMAIL_REGEX"
	RegexSentinelWinPath = "WIN_FILEPATH_REGEX"
)

func ExpandRegex(nameOrPattern string) (*regexp.Regexp, error) {
	nameOrPattern = strings.TrimSpace(nameOrPattern)
	switch nameOrPattern {
	case RegexSentinelEmail:
		return emailRegex, nil
	case RegexSentinelWinPath:
		return winFilepathRegex, nil
	case "":
		return nil, nil
	default:
		return regexp.Compile(nameOrPattern)
	}
}
