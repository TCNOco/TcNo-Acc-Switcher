package battlenetauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
)

var ErrAccountVerification = errors.New("Battle.net requires more account setup; check Account security or the Battle.net mobile app")

type Client struct{ http *http.Client }

func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 20 * time.Second, Transport: appclient.OfflineTransport(nil), CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("authenticator API redirect refused") }}}
}

func (c *Client) request(ctx context.Context, endpoint, bearer, contentType string, body []byte, preserve ...func([]byte) error) (map[string]json.RawMessage, error) {
	return c.requestMethod(ctx, http.MethodPost, endpoint, bearer, contentType, body, preserve...)
}

func (c *Client) requestMethod(ctx context.Context, method, endpoint, bearer, contentType string, body []byte, preserve ...func([]byte) error) (map[string]json.RawMessage, error) {
	if appclient.IsOfflineMode() {
		return nil, appclient.ErrOfflineMode
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, ErrInvalid
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := c.http.Do(req)
	if err != nil {
		if len(preserve) > 0 && preserve[0] != nil {
			if saveErr := preserve[0](nil); saveErr != nil {
				return nil, saveErr
			}
		}
		return nil, errors.New("Battle.net authenticator request failed; its outcome may be unknown, so enrollment was not retried")
	}
	defer response.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	defer clear(raw)
	if len(preserve) > 0 && preserve[0] != nil {
		if err := preserve[0](raw); err != nil {
			return nil, err
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Battle.net authenticator request returned HTTP %d", response.StatusCode)
	}
	if readErr != nil || len(raw) > maxResponseSize {
		return nil, ErrInvalid
	}
	o, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func accountSetupRequired(o map[string]json.RawMessage) bool {
	var required bool
	_ = json.Unmarshal(o["requireHealup"], &required)
	return required
}

func (c *Client) Enroll(ctx context.Context, sessionToken string) (Account, error) {
	return c.EnrollWithRecovery(ctx, sessionToken, nil)
}

func (c *Client) EnrollWithRecovery(ctx context.Context, sessionToken string, preserve func([]byte) error, identityReady ...func(EnrollmentIdentity) error) (Account, error) {
	if !validSessionToken(sessionToken) {
		return Account{}, ErrInvalid
	}
	body := []byte(url.Values{"client_id": {"baedda12fe054e4abdfc3ad7bdea970a"}, "grant_type": {"client_sso"}, "scope": {"auth.authenticator"}, "token": {sessionToken}}.Encode())
	defer clear(body)
	o, err := c.request(ctx, "https://oauth.battle.net/oauth/sso", "", "application/x-www-form-urlencoded", body)
	if err != nil {
		return Account{}, err
	}
	if accountSetupRequired(o) {
		return Account{}, ErrAccountVerification
	}
	bearer := textField(o, "access_token")
	if bearer == "" || len(bearer) > 16384 || strings.ContainsAny(bearer, "\r\n") {
		return Account{}, ErrInvalid
	}
	user, err := c.requestMethod(ctx, http.MethodGet, "https://oauth.battle.net/userinfo", bearer, "application/json", nil)
	if err != nil {
		return Account{}, fmt.Errorf("%w: %v", ErrAccountIdentity, err)
	}
	identity := EnrollmentIdentity{BattleTag: strings.TrimSpace(textField(user, "battletag")), Email: NormalizeAccountEmail(textField(user, "email"))}
	if err := identity.Validate(); err != nil {
		return Account{}, err
	}
	for _, ready := range identityReady {
		if ready != nil {
			if err := ready(identity); err != nil {
				return Account{}, err
			}
		}
	}
	o, err = c.request(ctx, "https://authenticator-rest-api.bnet-identity.blizzard.net/v1/authenticator", bearer, "application/json", nil, preserve)
	if err != nil {
		return Account{}, err
	}
	a, err := parseExternalObject(o)
	if err != nil && setupOnlyResponse(o) {
		return Account{}, ErrAccountVerification
	}
	a.Pending = true
	a.AccountName = identity.BattleTag
	a.AccountLink = identity.Email
	return a, err
}

func (c *Client) Restore(ctx context.Context, accountIdentifier, serial, restoreCode string) (Account, error) {
	return c.RestoreWithRecovery(ctx, accountIdentifier, serial, restoreCode, nil)
}

func (c *Client) RestoreWithRecovery(ctx context.Context, accountIdentifier, serial, restoreCode string, preserve func([]byte) error) (Account, error) {
	var err error
	serial, err = NormalizeSerial(serial)
	if err != nil || len(accountIdentifier) == 0 || len(accountIdentifier) > 320 || len(restoreCode) != 10 {
		return Account{}, ErrInvalid
	}
	for _, c := range accountIdentifier {
		if c < 32 || c == 127 {
			return Account{}, ErrInvalid
		}
	}
	body, err := json.Marshal(map[string]string{"accountIdentifier": accountIdentifier, "serial": serial, "restoreCode": restoreCode})
	if err != nil {
		return Account{}, ErrInvalid
	}
	defer clear(body)
	o, err := c.request(ctx, "https://authenticator-rest-api.bnet-identity.blizzard.net/v2/authenticator/device", "", "application/json", body, preserve)
	if err != nil {
		return Account{}, err
	}
	a := Account{Version: 1, Serial: serial, AccountName: strings.TrimSpace(accountIdentifier), AccountLink: strings.TrimSpace(accountIdentifier), RestoreCode: restoreCode, DeviceSecret: textField(o, "deviceSecret"), Pending: true, RequiresAccountSetup: accountSetupRequired(o)}
	if !strings.Contains(a.AccountLink, "@") {
		a.AccountLink = ""
	}
	if err := a.Validate(); err != nil {
		if setupOnlyResponse(o) {
			return Account{}, ErrAccountVerification
		}
		return Account{}, err
	}
	return a, nil
}

func validSessionToken(token string) bool {
	if len(token) < 20 || len(token) > 4096 {
		return false
	}
	region, _, ok := strings.Cut(token, "-")
	if !ok || !strings.Contains("|US|EU|KR|TW|CN|", "|"+region+"|") {
		return false
	}
	for _, c := range token {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func SessionTokenFromRedirect(raw string) (string, error) {
	if len(raw) > 8192 {
		return "", ErrInvalid
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Fragment != "" {
		return "", ErrInvalid
	}
	localCallback := u.Scheme == "http" && u.Host == "localhost"
	battleNetCallback := u.Scheme == "https" && u.Host == "account.battle.net" &&
		(u.EscapedPath() == "/login/en/localhost" || u.EscapedPath() == "/login/en/localhost/")
	if !localCallback && !battleNetCallback {
		return "", ErrInvalid
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(query["ST"]) != 1 {
		return "", ErrInvalid
	}
	token := query.Get("ST")
	if !validSessionToken(token) {
		return "", ErrInvalid
	}
	return token, nil
}
