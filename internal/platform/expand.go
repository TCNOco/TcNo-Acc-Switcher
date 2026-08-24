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

type pathToken struct {
	name  string
	value string
}

// userHomeDir resolves the home directory the placeholder tables are built on.
//
// USERPROFILE is preferred over os.UserHomeDir even off Windows, and both
// pathTokens and the cache-path safety check must call this: the two have to
// agree or a delete aimed at the real Desktop passes the check meant to stop it.
// Off Windows USERPROFILE is unset, so without the fallback every
// %UserProfile%-rooted path would fail the safety check as having no base.
func userHomeDir() string {
	home := strings.TrimSpace(os.Getenv("USERPROFILE"))
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}
	return home
}

// pathTokens resolves the placeholder table for the running OS. An empty value
// means this OS has no such folder.
func pathTokens() []pathToken {
	home := userHomeDir()
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

// ExpandWindowsPath resolves a catalog path's %Name% placeholders and rewrites
// separators off Windows.
//
// A path naming a placeholder this OS has no value for comes back empty: callers
// read "" as "no path", where a leftover "%ProgramFiles(x86)%\Steam\steam.exe"
// looks like a relative directory. Context tokens such as %Platform_Folder% are
// left for [ExpandPathTokens].
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
