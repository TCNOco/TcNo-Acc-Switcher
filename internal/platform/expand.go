package platform

import (
	"os"
	"path/filepath"
	"regexp"
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

func ExpandWindowsPath(s string) string {
	if s == "" {
		return ""
	}
	s = strings.TrimSpace(s)
	up := os.Getenv("USERPROFILE")
	m := map[string]string{
		"%ProgramFiles%":         os.Getenv("ProgramFiles"),
		"%ProgramFiles(x86)%":    os.Getenv("ProgramFiles(x86)"),
		"%LocalAppData%":         os.Getenv("LocalAppData"),
		"%AppData%":              os.Getenv("AppData"),
		"%UserProfile%":          up,
		"%USERPROFILE%":          up,
		"%Desktop%":              filepath.Join(up, "Desktop"),
		"%Documents%":            filepath.Join(up, "Documents"),
		"%Music%":                filepath.Join(up, "Music"),
		"%Pictures%":             filepath.Join(up, "Pictures"),
		"%Videos%":               filepath.Join(up, "Videos"),
		"%ProgramData%":          os.Getenv("ProgramData"),
		"%StartMenuAppData%":     filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`),
		"%StartMenuProgramData%": filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs`),
	}
	out := s
	for k, v := range m {
		if v != "" {
			out = strings.ReplaceAll(out, k, v)
		}
	}
	if lt := strings.ToLower(strings.TrimSpace(out)); strings.HasPrefix(lt, "http://") || strings.HasPrefix(lt, "https://") {
		return out
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
	emailRegex      = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
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
