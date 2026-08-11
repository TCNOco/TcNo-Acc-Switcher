package steambrowser

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantHost    string
		wantSecure  bool
		wantTrusted bool
	}{
		{
			name: "community profile", url: "https://steamcommunity.com/id/SomeUser/",
			wantHost: "steamcommunity.com", wantSecure: true, wantTrusted: true,
		},
		{
			name: "store subdomain", url: "https://store.steampowered.com/app/730/",
			wantHost: "store.steampowered.com", wantSecure: true, wantTrusted: true,
		},
		{
			name: "help subdomain", url: "https://help.steampowered.com/en/",
			wantHost: "help.steampowered.com", wantSecure: true, wantTrusted: true,
		},
		{
			name: "uppercase host with root dot", url: "https://STEAMCOMMUNITY.COM./market/",
			wantHost: "steamcommunity.com", wantSecure: true, wantTrusted: true,
		},
		{
			name: "explicit port", url: "https://steamcommunity.com:443/market/",
			wantHost: "steamcommunity.com", wantSecure: true, wantTrusted: true,
		},
		{
			name: "off-whitelist", url: "https://example.com/",
			wantHost: "example.com", wantSecure: true, wantTrusted: false,
		},
		{
			// http on a listed domain is still not safe to badge as trusted.
			name: "plain http on a listed domain", url: "http://steamcommunity.com/",
			wantHost: "steamcommunity.com", wantSecure: false, wantTrusted: false,
		},
		{
			name: "unparseable", url: "::::",
			wantHost: "", wantSecure: false, wantTrusted: false,
		},
		{
			name: "no host", url: "about:blank",
			wantHost: "", wantSecure: false, wantTrusted: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Classify(PlatformSteam, test.url)
			if got.Host != test.wantHost || got.Secure != test.wantSecure || got.Trusted != test.wantTrusted {
				t.Errorf("Classify(%q) = {host:%q secure:%v trusted:%v}, want {host:%q secure:%v trusted:%v}",
					test.url, got.Host, got.Secure, got.Trusted,
					test.wantHost, test.wantSecure, test.wantTrusted)
			}
		})
	}
}

// TestClassifyRejectsLookalikeHosts covers the ways a naive match would badge an
// attacker's page as Steam. Each of these is trusted under a substring test, a
// bare-suffix test, or both.
func TestClassifyRejectsLookalikeHosts(t *testing.T) {
	lookalikes := []string{
		"https://notsteampowered.com/",              // bare suffix, no dot separator
		"https://steampowered.com.evil.tld/",        // domain as a subdomain of something else
		"https://evil.tld/?next=steamcommunity.com", // domain only in the query
		"https://steamcommunity.com.attacker.net/",
		"https://xsteamcommunity.com/",
		"https://steamcommunity.co/",
		"https://steamcommunity.com@evil.tld/", // userinfo, host is evil.tld
	}
	for _, raw := range lookalikes {
		if got := Classify(PlatformSteam, raw); got.Trusted {
			t.Errorf("Classify(%q) reported trusted (host %q), want untrusted", raw, got.Host)
		}
	}
}

func TestClassifyUnknownPlatformTrustsNothing(t *testing.T) {
	if Classify("Epic", "https://steamcommunity.com/").Trusted {
		t.Error("an unconfigured platform trusted a Steam domain")
	}
}

func TestTrustedDomainsCopyIsIndependent(t *testing.T) {
	domains := TrustedDomains(PlatformSteam)
	if len(domains) == 0 {
		t.Fatal("Steam has no trusted domains")
	}
	domains[0] = "evil.tld"
	if IsTrusted(PlatformSteam, "https://evil.tld/") {
		t.Error("mutating the returned slice changed the trust list")
	}
}
