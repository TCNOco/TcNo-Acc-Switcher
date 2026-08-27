package steam

import "testing"

func TestExtractMiniprofileAvatarMediaURL_video(t *testing.T) {
	t.Parallel()

	fragment := `<div class="miniprofile_container">
<div class="playerAvatarAutoSizeInner">
<video class="avatar" autoplay muted loop playsinline>
<source src="https://cdn.akamai.steamstatic.com/steamcommunity/public/images/items/foo/abc.webm" type="video/webm"/>
</video>
</div>
</div>`

	got := ExtractMiniprofileAvatarMediaURL(fragment)
	want := "https://cdn.akamai.steamstatic.com/steamcommunity/public/images/items/foo/abc.webm"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestExtractMiniprofileAvatarMediaURL_gif(t *testing.T) {
	t.Parallel()

	fragment := `<div class="miniprofile_container">
<div class="miniprofile_playersection">
<div class="playersection_avatar border_color_online">
<img src="https://shared.akamai.steamstatic.com/community_assets/images/items/2599270/abc.gif"/>
</div>
</div>
</div>`

	got := ExtractMiniprofileAvatarMediaURL(fragment)
	want := "https://shared.akamai.steamstatic.com/community_assets/images/items/2599270/abc.gif"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// A still avatar's img is the same picture the full-quality static download
// already fetches; reporting it doubled the download and showed the worse copy.
func TestExtractMiniprofileAvatarMediaURL_stillImageIsNotMedia(t *testing.T) {
	t.Parallel()

	fragment := `<div class="miniprofile_container">
<div class="miniprofile_playersection">
<div class="playersection_avatar border_color_offline">
<img src="https://avatars.akamai.steamstatic.com/10b3fa00991a8b1115df292256f5fd2d1240a6d6_medium.jpg"/>
</div>
</div>
</div>`

	if got := ExtractMiniprofileAvatarMediaURL(fragment); got != "" {
		t.Fatalf("still avatar reported as media: %q", got)
	}
}

// A cached fragment has had its asset URLs rewritten to the app's own origin. Those
// paths are safe to render but are not fetchable, and the sole consumer of this
// function downloads whatever it returns.
func TestExtractMiniprofileAvatarMediaURL_localizedPathIsNotMedia(t *testing.T) {
	t.Parallel()

	gifImg := `<div class="miniprofile_container">
<div class="miniprofile_playersection">
<div class="playersection_avatar border_color_online">
<img src="/img/profiles/steam/76561198044414346_mini.gif"/>
</div>
</div>
</div>`
	if got := ExtractMiniprofileAvatarMediaURL(gifImg); got != "" {
		t.Fatalf("localized gif path reported as downloadable media: %q", got)
	}

	video := `<div class="miniprofile_container">
<div class="playerAvatarAutoSizeInner">
<video class="avatar" autoplay muted loop playsinline>
<source src="/img/profiles/steam/76561198044414346_mini.webm" type="video/webm"/>
</video>
</div>
</div>`
	if got := ExtractMiniprofileAvatarMediaURL(video); got != "" {
		t.Fatalf("localized video path reported as downloadable media: %q", got)
	}

	videoSrc := `<div class="miniprofile_container">
<div class="playerAvatarAutoSizeInner">
<video class="avatar" src="/img/profiles/steam/76561198044414346_mini.webm"></video>
</div>
</div>`
	if got := ExtractMiniprofileAvatarMediaURL(videoSrc); got != "" {
		t.Fatalf("localized video src reported as downloadable media: %q", got)
	}
}

func TestExtractMiniprofileDisplayName(t *testing.T) {
	t.Parallel()

	fragment := `<div class="miniprofile_container">
<div class="player_content"><span class="persona online">Jack Hoffman</span></div>
</div>`

	if got := ExtractMiniprofileDisplayName(fragment); got != "Jack Hoffman" {
		t.Fatalf("got %q want Jack Hoffman", got)
	}
}
