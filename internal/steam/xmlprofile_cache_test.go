package steam

import (
	"context"
	"errors"
	"os"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
)

const xmlCacheTestSteamID = "76561198000000000"

// A captive portal or an ISP interception page answers 200 with something that
// is not a profile. Reading it as one produced a blank account rather than a
// failure worth retrying.
func TestParseProfileXMLDocRejectsNonProfileBodies(t *testing.T) {
	for name, body := range map[string]string{
		"html":     "<html><body>Sign in to continue</body></html>",
		"empty":    "",
		"trunc":    "<profile><steamID64>7656119",
		"otherXML": "<error>nope</error>",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseProfileXMLDoc([]byte(body))
			var bodyErr *profileXMLBodyError
			if !errors.As(err, &bodyErr) {
				t.Fatalf("err = %v, want a profileXMLBodyError", err)
			}
			if !isTransientProfileRefreshError(err) {
				t.Fatal("an unreadable body must be retried, not shown as the account's state")
			}
		})
	}
}

func TestParseProfileXMLDocAcceptsAProfileAndAPrivateOne(t *testing.T) {
	doc, err := parseProfileXMLDoc([]byte(`<profile><steamID64>` + xmlCacheTestSteamID + `</steamID64><steamID>Name</steamID></profile>`))
	if err != nil {
		t.Fatalf("public profile: %v", err)
	}
	if doc.SteamCommunityTitle != "Name" {
		t.Fatalf("title = %q, want Name", doc.SteamCommunityTitle)
	}
	if _, err := parseProfileXMLDoc([]byte(`<profile><privacyMessage>hidden</privacyMessage></profile>`)); err != nil {
		t.Fatalf("private profile: %v", err)
	}
}

// The cache is inside its 24h lifetime here, so nothing goes to the network:
// the only way out is dropping the poisoned copy, or every retry re-reads it.
func TestFetchProfileXMLDropsACachedBodyThatStoppedParsing(t *testing.T) {
	paths.ResetForTest(t.TempDir())
	cache, err := xmlCachePath(xmlCacheTestSteamID)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, cache, "<html>not a profile</html>")

	if _, err := FetchProfileXML(context.Background(), nil, xmlCacheTestSteamID); err == nil {
		t.Fatal("a cached body that does not parse must be reported, not returned as fields")
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("the poisoned cache survived (stat err = %v), so every retry would re-read it", err)
	}
}
