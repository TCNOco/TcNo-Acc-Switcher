package platform

import "testing"

// A descriptor's QuitArgs is the only thing standing between a launcher that ignores WM_CLOSE
// and a graceful window that expires on every switch, so the catalog-to-code wiring is worth
// pinning: a rename or a dropped field would regress silently rather than fail a build.
func TestDescriptorQuitArgsParse(t *testing.T) {
	raw := []byte(`{"Platforms":{
		"WithQuit":{"ExesToEnd":["a.exe"],"Extras":{"QuitArgs":"-shutdown"}},
		"MultiArg":{"ExesToEnd":["b.exe"],"Extras":{"QuitArgs":"  --quit  --now "}},
		"NoQuit":{"ExesToEnd":["c.exe"],"Extras":{"CachePaths":["x"]}}
	}}`)

	for _, tc := range []struct {
		key  string
		want []string
	}{
		{"WithQuit", []string{"-shutdown"}},
		{"MultiArg", []string{"--quit", "--now"}},
		{"NoQuit", nil},
	} {
		d, err := ParseDescriptor(raw, tc.key)
		if err != nil {
			t.Fatalf("%s: ParseDescriptor: %v", tc.key, err)
		}
		got := LaunchArgTokens(d.Extras.QuitArgs)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: got %q, want %q", tc.key, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("%s: got %q, want %q", tc.key, got, tc.want)
			}
		}
	}
}
