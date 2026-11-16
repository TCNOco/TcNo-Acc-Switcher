package platforms

import (
	"path/filepath"
	"testing"
)

func TestParseCachedJSONValue(t *testing.T) {
	for _, tc := range []struct {
		name, data, path, matchPath, matchValue, want string
	}{
		{"binary envelope", "\x00cache\x00" + `{"userId":"current","name":"Player"}` + "\x00trailer", "name", "userId", "current", "Player"},
		{"other account first", `{"userId":"other","name":"Wrong"}` + "\x00" + `{"userId":"current","name":"Right"}`, "name", "userId", "current", "Right"},
		{"missing current account", `{"userId":"other","name":"Wrong"}`, "name", "userId", "current", ""},
		{"no cross-record fields", `{"userId":"current"}` + "\x00" + `{"userId":"other","name":"Wrong"}`, "name", "userId", "current", ""},
		{"nested fields", `{"profile":{"id":"current","name":"Nested Player"}}`, "profile.name", "profile.id", "current", "Nested Player"},
		{"JSON escapes", `{"name":"  Player \"One\" \u2603  "}`, "name", "", "", "Player \"One\" ☃"},
		{"malformed header", `header{broken` + "\x00" + `{"name":"Recovered"}`, "name", "", "", "Recovered"},
		{"truncated object", `{"name":"Incomplete"`, "name", "", "", ""},
		{"numeric name", `{"name":123}`, "name", "", "", ""},
		{"empty name", `{"name":"  "}`, "name", "", "", ""},
		{"numeric identity", `{"id":123,"name":"Wrong"}`, "name", "id", "123", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseCachedJSONValue([]byte(tc.data), tc.path, tc.matchPath, tc.matchValue); got != tc.want {
				t.Fatalf("got %q; want %q", got, tc.want)
			}
		})
	}
}

func TestCachedJSONValueFallbackAndAccountMatch(t *testing.T) {
	primary := filepath.Join("testdata", "cache_providers", "geforce_now.txt")
	fallback := filepath.Join("testdata", "cache_providers", "json_profile.txt")
	name, err := CachedJSONValue(primary, []string{fallback}, "nameOnPlatform", "userId", "current-user")
	if err != nil || name != "CurrentPlayer" {
		t.Fatalf("name = %q, %v", name, err)
	}
	name, err = CachedJSONValue(primary, []string{fallback}, "nameOnPlatform", "userId", "unknown-user")
	if err != nil || name != "" {
		t.Fatalf("another user's profile must not be returned: %q, %v", name, err)
	}
	if _, err := CachedJSONValue(primary, nil, "nameOnPlatform", "userId", ""); err == nil {
		t.Fatal("missing current ID must not disable filtering")
	}
	if _, err := CachedJSONValue(primary, nil, "", "", ""); err == nil {
		t.Fatal("missing selector should be rejected")
	}
}
