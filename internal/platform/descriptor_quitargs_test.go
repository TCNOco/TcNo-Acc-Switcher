package platform

import "testing"

// QuitArgs is the only way to close a launcher that ignores WM_CLOSE, and a renamed or dropped field regresses silently rather than failing the build.
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
