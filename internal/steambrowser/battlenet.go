package steambrowser

import (
	"errors"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func OpenBattleNetLogin(s *Service, profile string, intercept func(string) bool) error {
	return openBattleNetWindow(s, profile, "https://account.battle.net/login/en/?ref=localhost", Site("battlenet-login"), intercept)
}

func OpenBattleNetAccount(s *Service, profile string) error {
	return openBattleNetWindow(s, profile, "https://account.battle.net/security", Site("battlenet-account"), func(raw string) bool { return !Classify(PlatformBattleNet, raw).Trusted })
}

func openBattleNetWindow(s *Service, profile, destination string, site Site, intercept func(string) bool) error {
	if s == nil || !Supported() || intercept == nil {
		return ErrUnsupportedPlatform
	}
	if !s.Available(){return ErrStorageUnavailable}
	id, err := s.sessions.reserve()
	if err != nil {
		return err
	}
	var openErr error
	application.InvokeSync(func() {
		openErr = s.openOnMainThread(id, WebSession{AccountName: "Battle.net"}, site, destination, profile, nil, intercept)
	})
	if openErr != nil {
		s.sessions.release(id)
		return errors.New("Battle.net login window could not be opened")
	}
	return nil
}
