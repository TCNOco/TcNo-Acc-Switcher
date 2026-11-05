package steambrowser

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed battlenet_account_name.js
var battleNetAccountNameScript string

//go:embed battlenet_login_identity.js
var battleNetLoginIdentityScript string

var errBattleNetAccountName = errors.New("Battle.net account name is unavailable")
var battleNetAccountHost = regexp.MustCompile(`^(?:(?:eu|us|kr|tw|cn)\.)?account\.battle\.net$`)
var battleNetLoginPath = regexp.MustCompile(`^/login/(?:[a-z]{2}(?:-[a-z]{2})?/)?(?:password/?)?$`)

func battleNetLoginPage(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && battleNetAccountHost.MatchString(u.Host) && u.User == nil && battleNetLoginPath.MatchString(u.EscapedPath())
}

func battleNetSecurityPage(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && battleNetAccountHost.MatchString(u.Host) && u.User == nil && (u.EscapedPath() == "/security" || u.EscapedPath() == "/security/")
}

func parseBattleNetAccountName(raw string) (string, error) {
	identity, err := parseBattleNetAccountIdentity(raw)
	return identity.Name, err
}

type BattleNetAccountIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func parseBattleNetAccountIdentity(raw string) (BattleNetAccountIdentity, error) {
	return parseBattleNetIdentity(raw, battleNetSecurityPage)
}

func parseBattleNetLoginIdentity(raw string) (BattleNetAccountIdentity, error) {
	identity, err := parseBattleNetIdentity(raw, battleNetLoginPage)
	identity.Name = ""
	return identity, err
}

func parseBattleNetIdentity(raw string, allowed func(string) bool) (BattleNetAccountIdentity, error) {
	var empty BattleNetAccountIdentity
	if raw == "null" {
		return empty, nil
	}
	if len(raw) > 2048 {
		return empty, errBattleNetAccountName
	}
	var result struct {
		Source string `json:"source"`
		Name   string `json:"name"`
		Email  string `json:"email"`
	}
	if json.Unmarshal([]byte(raw), &result) != nil || !allowed(result.Source) {
		return empty, errBattleNetAccountName
	}
	name := strings.TrimSpace(result.Name)
	if len(name) > 320 || !utf8.ValidString(name) {
		return empty, errBattleNetAccountName
	}
	for _, c := range name {
		if unicode.IsControl(c) || unicode.Is(unicode.Cf, c) {
			return empty, errBattleNetAccountName
		}
	}
	email := strings.TrimSpace(result.Email)
	if email != "" {
		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email || len(email) > 320 || strings.ContainsAny(email, "*\r\n") {
			email = ""
		}
	}
	return BattleNetAccountIdentity{Name: name, Email: email}, nil
}

type battleNetAccountNameReader interface {
	readBattleNetAccountName(func(BattleNetAccountIdentity, error)) error
}

type battleNetLoginIdentityReader interface {
	readBattleNetLoginIdentity(func(BattleNetAccountIdentity, error)) error
}

func ReadBattleNetAccountIdentity(ctx context.Context, s *Service, profile string) (BattleNetAccountIdentity, error) {
	return readBattleNetIdentity(ctx, s, profile, false)
}

func ReadBattleNetLoginIdentity(ctx context.Context, s *Service, profile string) (BattleNetAccountIdentity, error) {
	return readBattleNetIdentity(ctx, s, profile, true)
}

func readBattleNetIdentity(ctx context.Context, s *Service, profile string, login bool) (BattleNetAccountIdentity, error) {
	if s == nil || application.Get() == nil {
		return BattleNetAccountIdentity{}, nil
	}
	type result struct {
		identity BattleNetAccountIdentity
		err      error
	}
	done := make(chan result, 1)
	complete := func(identity BattleNetAccountIdentity, err error) {
		select {
		case done <- result{identity, err}:
		default:
		}
	}
	application.InvokeAsync(func() {
		if ctx.Err() != nil {
			complete(BattleNetAccountIdentity{}, ctx.Err())
			return
		}
		for _, id := range s.sessions.ids() {
			current, err := s.sessions.get(id)
			if err != nil || current.platform != PlatformBattleNet || current.profile != profile {
				continue
			}
			if login {
				if reader, ok := current.view.(battleNetLoginIdentityReader); ok {
					if err := reader.readBattleNetLoginIdentity(complete); err != nil {
						complete(BattleNetAccountIdentity{}, err)
					}
					return
				}
				continue
			}
			if reader, ok := current.view.(battleNetAccountNameReader); ok {
				if err := reader.readBattleNetAccountName(complete); err != nil {
					complete(BattleNetAccountIdentity{}, err)
				}
				return
			}
		}
		complete(BattleNetAccountIdentity{}, nil)
	})
	select {
	case value := <-done:
		return value.identity, value.err
	case <-ctx.Done():
		return BattleNetAccountIdentity{}, ctx.Err()
	}
}
