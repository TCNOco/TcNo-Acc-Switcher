package steambrowser

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed battlenet_account_name.js
var battleNetAccountNameScript string

var errBattleNetAccountName = errors.New("Battle.net account name is unavailable")
var battleNetAccountHost = regexp.MustCompile(`^(?:(?:eu|us|kr|tw|cn)\.)?account\.battle\.net$`)

func battleNetSecurityPage(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && battleNetAccountHost.MatchString(u.Host) && u.User == nil && (u.EscapedPath() == "/security" || u.EscapedPath() == "/security/")
}

func parseBattleNetAccountName(raw string) (string, error) {
	if raw == "null" {
		return "", nil
	}
	if len(raw) > 2048 {
		return "", errBattleNetAccountName
	}
	var result struct {
		Source string `json:"source"`
		Name   string `json:"name"`
	}
	if json.Unmarshal([]byte(raw), &result) != nil || !battleNetSecurityPage(result.Source) {
		return "", errBattleNetAccountName
	}
	name := strings.TrimSpace(result.Name)
	if len(name) > 320 || !utf8.ValidString(name) {
		return "", errBattleNetAccountName
	}
	for _, c := range name {
		if unicode.IsControl(c) || unicode.Is(unicode.Cf, c) {
			return "", errBattleNetAccountName
		}
	}
	return name, nil
}

type battleNetAccountNameReader interface {
	readBattleNetAccountName(func(string, error)) error
}

func ReadBattleNetAccountName(ctx context.Context, s *Service, profile string) (string, error) {
	if s == nil || application.Get() == nil {
		return "", nil
	}
	type result struct {
		name string
		err  error
	}
	done := make(chan result, 1)
	complete := func(name string, err error) {
		select {
		case done <- result{name, err}:
		default:
		}
	}
	application.InvokeAsync(func() {
		if ctx.Err() != nil {
			complete("", ctx.Err())
			return
		}
		for _, id := range s.sessions.ids() {
			current, err := s.sessions.get(id)
			if err != nil || current.platform != PlatformBattleNet || current.profile != profile {
				continue
			}
			if reader, ok := current.view.(battleNetAccountNameReader); ok {
				if err := reader.readBattleNetAccountName(complete); err != nil {
					complete("", err)
				}
				return
			}
		}
		complete("", nil)
	})
	select {
	case value := <-done:
		return value.name, value.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
