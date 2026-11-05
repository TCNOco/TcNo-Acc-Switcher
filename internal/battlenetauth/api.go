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
)

var ErrAccountVerification = errors.New("Battle.net requires more account setup; check Account security or the Battle.net mobile app")

type Client struct{ http *http.Client }

func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("authenticator API redirect refused") }}}
}

func (c *Client) request(ctx context.Context, endpoint, bearer, contentType string, body []byte) (map[string]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
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
		return nil, errors.New("Battle.net authenticator request failed; its outcome may be unknown, so enrollment was not retried")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Battle.net authenticator request returned HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(raw) > 65536 {
		return nil, ErrInvalid
	}
	defer clear(raw)
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
	o, err = c.request(ctx, "https://authenticator-rest-api.bnet-identity.blizzard.net/v1/authenticator", bearer, "application/json", nil)
	if err != nil {
		return Account{}, err
	}
	a, err := parseExternalObject(o)
	if err != nil && accountSetupRequired(o) {
		return Account{}, ErrAccountVerification
	}
	a.Pending = true
	return a, err
}

func (c *Client) Restore(ctx context.Context, accountIdentifier, serial, restoreCode string) (Account, error) {
	if len(accountIdentifier) == 0 || len(accountIdentifier) > 320 || !serialPattern.MatchString(serial) || len(restoreCode) != 10 {
		return Account{}, ErrInvalid
	}
	body, err := json.Marshal(map[string]string{"accountIdentifier": accountIdentifier, "serial": serial, "restoreCode": restoreCode})
	if err != nil {
		return Account{}, ErrInvalid
	}
	defer clear(body)
	o, err := c.request(ctx, "https://authenticator-rest-api.bnet-identity.blizzard.net/v2/authenticator/device", "", "application/json", body)
	if err != nil {
		return Account{}, err
	}
	a := Account{Version: 1, Serial: serial, RestoreCode: restoreCode, DeviceSecret: textField(o, "deviceSecret"), Pending: true, RequiresAccountSetup: accountSetupRequired(o)}
	if err := a.Validate(); err != nil {
		if a.RequiresAccountSetup {
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
