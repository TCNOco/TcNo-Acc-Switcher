package logredact_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/logredact"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/qr"
)

func TestRedactTextRemovesLabelledSecretsAndQRChallenges(t *testing.T) {
	sentinels := []string{
		"PASSWORD_SENTINEL", "SHARED_SENTINEL", "IDENTITY_SENTINEL",
		"REVOCATION_SENTINEL", "TOKEN_SENTINEL", "SESSION_SENTINEL",
		"CHALLENGE_SENTINEL", "76543210987654321",
	}
	input := `password=PASSWORD_SENTINEL shared_secret="SHARED_SENTINEL" ` +
		`identity-secret:'IDENTITY_SENTINEL' revocation_code=REVOCATION_SENTINEL ` +
		`AccessToken=TOKEN_SENTINEL SessionID=SESSION_SENTINEL ` +
		`challenge=CHALLENGE_SENTINEL https://s.team/q/1/76543210987654321`
	output := logredact.RedactText(input)
	assertAbsent(t, output, sentinels...)
}

func TestHandlerRedactsStructuredSteamGuardValuesAndPreservesIdentifiers(t *testing.T) {
	const (
		shared   = "SHARED_STRUCT_SENTINEL"
		identity = "IDENTITY_STRUCT_SENTINEL"
		token    = "TOKEN_STRUCT_SENTINEL"
		wrapped  = "WRAPPED_ERROR_SENTINEL"
		steamID  = "76561198123456789"
		username = "ordinary_username"
	)
	account := mafile.Account{
		SharedSecret: shared, IdentitySecret: identity, AccountName: username,
		Session: &mafile.SessionData{AccessToken: token, SteamID: 76561198123456789},
	}
	challenge := qr.Challenge{Version: 1, ClientID: 998877665544332211}

	var output bytes.Buffer
	logger := slog.New(logredact.NewHandler(slog.NewTextHandler(&output, nil))).With("account", account)
	logger.Error("Steam Guard failed", "err", fmt.Errorf("wrapped: refresh_token=%s", wrapped), "qr", challenge, "accountID", steamID, "username", username)

	logged := output.String()
	assertAbsent(t, logged, shared, identity, token, wrapped, "998877665544332211")
	for _, public := range []string{steamID, username} {
		if !strings.Contains(logged, public) {
			t.Fatalf("public identifier %q was unexpectedly removed: %s", public, logged)
		}
	}
}

func assertAbsent(t *testing.T, output string, values ...string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(output, value) {
			t.Fatalf("secret sentinel %q leaked in %q", value, output)
		}
	}
}
