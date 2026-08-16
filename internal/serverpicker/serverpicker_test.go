package serverpicker

import (
	"testing"
)

// fixture mirrors the shape of a real GetSDRConfig response, trimmed to the
// cases that exercise parsing and grouping: co-located POPs, POPs that only
// look related, a POP with no relays, and a bad address.
const fixture = `{
  "revision": 1786739253,
  "pops": {
    "sto":  {"desc": "Stockholm - Kista (Sweden)",  "geo": [17.9, 59.4],
             "relays": [{"ipv4": "185.25.182.1"}, {"ipv4": "185.25.182.2"}]},
    "sto2": {"desc": "Stockholm - Bromma (Sweden)", "geo": [17.87, 59.34],
             "relays": [{"ipv4": "155.133.226.1"}]},
    "pvgm": {"desc": "Alibaba Cloud Shanghai - Mobile (China)",  "geo": [121.5, 31.2],
             "relays": [{"ipv4": "203.107.54.1"}]},
    "pvgt": {"desc": "Alibaba Cloud Shanghai - Telecom (China)", "geo": [121.5, 31.2],
             "relays": [{"ipv4": "203.107.54.2"}]},
    "pvgu": {"desc": "Alibaba Cloud Shanghai - Unicom (China)",  "geo": [121.5, 31.2],
             "relays": [{"ipv4": "203.107.54.3"}]},
    "bom2": {"desc": "Mumbai (India)",  "geo": [72.99, 19.18], "relays": [{"ipv4": "103.10.124.1"}]},
    "maa2": {"desc": "Chennai - Ambattur (India)", "geo": [80.15, 13.11], "relays": [{"ipv4": "103.28.54.1"}]},
    "atl":  {"desc": "Atlanta (Georgia)", "geo": [-84.39, 33.76], "relays": [{"ipv4": "162.254.199.170"}]},
    "hkg":  {"desc": "Hong Kong", "geo": [113.92, 22.31], "relays": [{"ipv4": "103.10.125.1"}]},
    "pvg":  {"desc": "Alibaba Cloud Shanghai (China)", "geo": [121.5, 31.2]},
    "junk": {"desc": "Nowhere (Nothing)", "geo": [0, 0], "relays": [{"ipv4": "not-an-ip"}]}
  }
}`

func mustParse(t *testing.T) SDRConfig {
	t.Helper()
	cfg, err := parseSDRConfig([]byte(fixture))
	if err != nil {
		t.Fatalf("parseSDRConfig: %v", err)
	}
	return cfg
}

func TestParseSDRConfigDropsPOPsWithNothingToBlock(t *testing.T) {
	cfg := mustParse(t)

	if cfg.Revision != 1786739253 {
		t.Errorf("revision = %d, want 1786739253", cfg.Revision)
	}
	for _, pop := range cfg.POPs {
		switch pop.ID {
		case "pvg":
			t.Error("kept a POP with no relays; it exposes no addresses to block")
		case "junk":
			t.Error("kept a POP whose only relay address does not parse")
		}
	}
	if got := len(cfg.POPs); got != 9 {
		t.Errorf("kept %d POPs, want 9", got)
	}
	if cfg.POPs[0].ID != "atl" {
		t.Errorf("POPs are not sorted by id: first is %q", cfg.POPs[0].ID)
	}
}

func groupByID(t *testing.T, groups []Group, id string) Group {
	t.Helper()
	for _, g := range groups {
		if g.ID == id {
			return g
		}
	}
	t.Fatalf("no group with id %q", id)
	return Group{}
}

func TestBuildGroupsMergesCoLocatedPOPs(t *testing.T) {
	game, _ := gameByID("deadlock")
	groups := buildGroups(mustParse(t), game)

	stockholm := groupByID(t, groups, "sto")
	if got := stockholm.MemberIDs(); len(got) != 2 || got[0] != "sto" || got[1] != "sto2" {
		t.Errorf("Stockholm members = %v, want [sto sto2]; blocking one alone lets Steam route through the other", got)
	}
	if got := len(stockholm.RelayIPs()); got != 3 {
		t.Errorf("Stockholm covers %d relay IPs, want all 3 from both POPs", got)
	}

	shanghai := groupByID(t, groups, "pvgm")
	if got := len(shanghai.MemberIDs()); got != 3 {
		t.Errorf("Shanghai members = %v, want the Mobile/Telecom/Unicom trio", shanghai.MemberIDs())
	}
}

func TestBuildGroupsKeepsDistantCitiesApart(t *testing.T) {
	game, _ := gameByID("deadlock")
	groups := buildGroups(mustParse(t), game)

	mumbai := groupByID(t, groups, "bom2")
	chennai := groupByID(t, groups, "maa2")
	if len(mumbai.Members) != 1 || len(chennai.Members) != 1 {
		t.Errorf("Mumbai and Chennai merged (%v / %v); they are ~1000 km apart and differ in ping",
			mumbai.MemberIDs(), chennai.MemberIDs())
	}
}

func TestGroupLabelNamesThePlaceNotAMember(t *testing.T) {
	game, _ := gameByID("deadlock")
	groups := buildGroups(mustParse(t), game)

	tests := []struct {
		groupID string
		want    string
	}{
		{"sto", "Stockholm (Sweden)"},
		{"pvgm", "Alibaba Cloud Shanghai (China)"},
		{"hkg", "Hong Kong"},
		{"bom2", "Mumbai (India)"},
	}
	for _, tt := range tests {
		if got := groupByID(t, groups, tt.groupID).Label; got != tt.want {
			t.Errorf("group %s label = %q, want %q", tt.groupID, got, tt.want)
		}
	}
}

func TestBuildGroupsResolvesCountryAndRegion(t *testing.T) {
	game, _ := gameByID("deadlock")
	groups := buildGroups(mustParse(t), game)

	tests := []struct {
		groupID string
		country string
		region  string
	}{
		{"sto", "se", RegionEurope},
		{"pvgm", "cn", RegionAsia},
		{"hkg", "hk", RegionAsia},
		// Steam names the US state, not the country.
		{"atl", "us", RegionNorthAmerica},
	}
	for _, tt := range tests {
		g := groupByID(t, groups, tt.groupID)
		if g.Country != tt.country || g.Region != tt.region {
			t.Errorf("group %s = (%q, %q), want (%q, %q)", tt.groupID, g.Country, g.Region, tt.country, tt.region)
		}
	}
}

func TestGameKeywordFilterSplitsTheTwoCS2Entries(t *testing.T) {
	cfg := mustParse(t)

	cs2, _ := gameByID("cs2")
	for _, g := range buildGroups(cfg, cs2) {
		if g.Country == "cn" {
			t.Errorf("CS2 listed the China-only POP group %q", g.ID)
		}
	}

	pw, _ := gameByID("cs2_perfect_world")
	pwGroups := buildGroups(cfg, pw)
	if len(pwGroups) != 1 || pwGroups[0].Country != "cn" {
		t.Errorf("CS2 Perfect World groups = %d, want only the China group", len(pwGroups))
	}
}

func TestRegionFromGeoCoversEachContinent(t *testing.T) {
	tests := []struct {
		name     string
		lon, lat float64
		want     string
	}{
		{"Amsterdam", 4.9, 52.37, RegionEurope},
		{"Chicago", -87.69, 41.84, RegionNorthAmerica},
		{"Sao Paulo", -46.64, -23.53, RegionSouthAmerica},
		{"Johannesburg", 28, -26.2, RegionAfrica},
		{"Dubai", 55.3, 25.25, RegionMiddleEast},
		{"Tokyo", 139.68, 35.68, RegionAsia},
		{"Sydney", 151.21, -33.86, RegionOceania},
		{"unknown", 0, 0, ""},
	}
	for _, tt := range tests {
		if got := regionFromGeo(tt.lon, tt.lat); got != tt.want {
			t.Errorf("%s: regionFromGeo(%v, %v) = %q, want %q", tt.name, tt.lon, tt.lat, got, tt.want)
		}
	}
}

func TestLossPercentReportsPartialLoss(t *testing.T) {
	tests := []struct {
		successes int
		want      float64
	}{
		{0, 100},
		{1, 75},
		{2, 50},
		{3, 25},
		{4, 0},
	}
	for _, tt := range tests {
		if got := lossPercent(tt.successes, 4); got != tt.want {
			t.Errorf("lossPercent(%d, 4) = %v, want %v", tt.successes, got, tt.want)
		}
	}
}

func TestNormalizeGroupIDsKeepsTheFileStable(t *testing.T) {
	got := normalizeGroupIDs([]string{" sto ", "hkg", "sto", "", "atl"})
	want := []string{"atl", "hkg", "sto"}
	if len(got) != len(want) {
		t.Fatalf("normalizeGroupIDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeGroupIDs = %v, want %v", got, want)
		}
	}
}
