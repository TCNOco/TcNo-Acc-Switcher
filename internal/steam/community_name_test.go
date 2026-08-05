package steam

import (
	"fmt"
	"strings"
	"testing"
)

// miniprofileFixture builds a fragment shaped like Steam's real miniprofile
// payload, with enough nesting and text that parsing it is not free.
func miniprofileFixture(persona string) string {
	var b strings.Builder
	b.WriteString(`<div class="miniprofile_container"><div class="miniprofile_header">`)
	// The extractor matches a <span> whose class starts with "persona"; a div
	// here would silently yield no name and make the benchmark meaningless.
	b.WriteString(`<span class="persona online">` + persona + `</span>`)
	b.WriteString(`<div class="miniprofile_avatar online"><img src="https://avatars.example/a.jpg"></div>`)
	b.WriteString(`</div><div class="miniprofile_content">`)
	for i := 0; i < 12; i++ {
		b.WriteString(fmt.Sprintf(`<div class="miniprofile_row"><span class="label">Row %d</span><span class="value">Some &amp; value %d</span></div>`, i, i))
	}
	b.WriteString(`<div class="miniprofile_gamesection"><div class="game">Dropped by the sanitiser</div></div>`)
	b.WriteString(`</div></div>`)
	return b.String()
}

// TestCommunityDisplayNameFromMatchesExtraction pins the split: passing HTML in
// must yield exactly what extracting from that HTML yields, so the account
// list's reuse cannot drift from the single-account path.
func TestCommunityDisplayNameFromMatchesExtraction(t *testing.T) {
	cases := []struct {
		name string
		html string
	}{
		{name: "persona present", html: miniprofileFixture("Wesley")},
		{name: "persona with entities", html: miniprofileFixture("A &amp; B")},
		{name: "empty html", html: ""},
		{name: "no persona element", html: `<div class="miniprofile_container"><div class="miniprofile_content">x</div></div>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := ExtractMiniprofileDisplayName(tc.html)
			got := communityDisplayNameFrom(tc.html, "76561198000000001")
			// With no XML cache present the fallback yields "", so the two must
			// agree on every input.
			if got != want {
				t.Fatalf("communityDisplayNameFrom = %q, want %q", got, want)
			}
		})
	}
}

// BenchmarkMiniprofileParsePerAccount compares the two shapes the account list
// can take for one account. Both end with the same two results — sanitised HTML
// for the DTO and a display name — so the only difference is whether the
// fragment is parsed and sanitised once or twice.
//
// File I/O is excluded; the old shape also re-read the file, so the real saving
// is this plus one os.ReadFile per account.
func BenchmarkMiniprofileParsePerAccount(b *testing.B) {
	raw := miniprofileFixture("Wesley")

	// Old: sanitise for the DTO, then CachedCommunityDisplayName sanitises the
	// same fragment again before extracting the name from it.
	b.Run("Before/SanitiseTwiceThenExtract", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			forDTO := sanitizeMiniprofileHTML(raw)
			forName := sanitizeMiniprofileHTML(raw)
			if forDTO == "" || ExtractMiniprofileDisplayName(forName) == "" {
				b.Fatal("produced nothing")
			}
		}
	})

	// New: sanitise once, extract the name from the copy already in hand.
	b.Run("After/SanitiseOnceThenExtract", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cleaned := sanitizeMiniprofileHTML(raw)
			if cleaned == "" || ExtractMiniprofileDisplayName(cleaned) == "" {
				b.Fatal("produced nothing")
			}
		}
	})

	// The individual costs, to show where the time actually goes.
	b.Run("Parts/Sanitise", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = sanitizeMiniprofileHTML(raw)
		}
	})
	b.Run("Parts/Extract", func(b *testing.B) {
		cleaned := sanitizeMiniprofileHTML(raw)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ExtractMiniprofileDisplayName(cleaned)
		}
	})
}
