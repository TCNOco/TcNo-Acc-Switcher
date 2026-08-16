package serverpicker

import (
	"math"
	"strings"
)

// Region buckets the UI's region dropdown offers.
const (
	RegionAsia         = "asia"
	RegionEurope       = "europe"
	RegionNorthAmerica = "northAmerica"
	RegionSouthAmerica = "southAmerica"
	RegionOceania      = "oceania"
	RegionAfrica       = "africa"
	RegionMiddleEast   = "middleEast"
)

// country maps a POP to a flag and a searchable country name.
type country struct {
	Code   string // ISO 3166-1 alpha-2, lowercase; "" means unknown
	Name   string
	Region string
}

// countryByPlace matches a place name appearing in a POP description. Steam
// writes the country in parentheses for most POPs ("Amsterdam (Netherlands)"),
// but city-states have none ("Hong Kong", "Singapore", "Guam") and US POPs name
// the state instead ("Atlanta (Georgia)"), so both spellings are listed.
var countryByPlace = map[string]country{
	// Europe
	"netherlands": {"nl", "Netherlands", RegionEurope},
	"germany":     {"de", "Germany", RegionEurope},
	"england":     {"gb", "United Kingdom", RegionEurope},
	"scotland":    {"gb", "United Kingdom", RegionEurope},
	"wales":       {"gb", "United Kingdom", RegionEurope},
	"spain":       {"es", "Spain", RegionEurope},
	"france":      {"fr", "France", RegionEurope},
	"sweden":      {"se", "Sweden", RegionEurope},
	"finland":     {"fi", "Finland", RegionEurope},
	"austria":     {"at", "Austria", RegionEurope},
	"poland":      {"pl", "Poland", RegionEurope},

	// North America (Steam names US states, not the country)
	"georgia":              {"us", "United States", RegionNorthAmerica},
	"texas":                {"us", "United States", RegionNorthAmerica},
	"virginia":             {"us", "United States", RegionNorthAmerica},
	"california":           {"us", "United States", RegionNorthAmerica},
	"illinois":             {"us", "United States", RegionNorthAmerica},
	"washington":           {"us", "United States", RegionNorthAmerica},
	"new york":             {"us", "United States", RegionNorthAmerica},
	"united states":        {"us", "United States", RegionNorthAmerica},
	"canada":               {"ca", "Canada", RegionNorthAmerica},
	"mexico":               {"mx", "Mexico", RegionNorthAmerica},

	// South America
	"argentina": {"ar", "Argentina", RegionSouthAmerica},
	"brazil":    {"br", "Brazil", RegionSouthAmerica},
	"peru":      {"pe", "Peru", RegionSouthAmerica},
	"chile":     {"cl", "Chile", RegionSouthAmerica},

	// Asia
	"china":       {"cn", "China", RegionAsia},
	"hong kong":   {"hk", "Hong Kong", RegionAsia},
	"india":       {"in", "India", RegionAsia},
	"south korea": {"kr", "South Korea", RegionAsia},
	"singapore":   {"sg", "Singapore", RegionAsia},
	"japan":       {"jp", "Japan", RegionAsia},

	// Oceania
	"australia":   {"au", "Australia", RegionOceania},
	"guam":        {"gu", "Guam", RegionOceania},
	"new zealand": {"nz", "New Zealand", RegionOceania},

	// Africa / Middle East
	"south africa":         {"za", "South Africa", RegionAfrica},
	"united arab emirates": {"ae", "United Arab Emirates", RegionMiddleEast},
}

// resolveCountry identifies a POP's country from its description, falling back
// to its coordinates. The description is authoritative because Steam writes it
// by hand; geo only has to produce a sensible region for a POP added upstream
// that this table has not learned yet.
func resolveCountry(pop POP) country {
	desc := strings.ToLower(pop.Desc)

	if open := strings.LastIndex(desc, "("); open != -1 {
		if close := strings.Index(desc[open:], ")"); close != -1 {
			place := strings.TrimSpace(desc[open+1 : open+close])
			if c, ok := countryByPlace[place]; ok {
				return c
			}
		}
	}
	// City-states and territories carry no parenthetical, so match on the whole
	// description. Longest key first, else "china" would win over "hong kong".
	best := country{}
	bestLen := 0
	for place, c := range countryByPlace {
		if len(place) > bestLen && strings.Contains(desc, place) {
			best, bestLen = c, len(place)
		}
	}
	if bestLen > 0 {
		return best
	}
	return country{Region: regionFromGeo(pop.Lon, pop.Lat)}
}

// regionFromGeo buckets coordinates into a continent. Coarse by design: it is
// the fallback for POPs whose country this build does not know, so "roughly
// right so the region filter still works" beats a precise boundary table.
func regionFromGeo(lon, lat float64) string {
	switch {
	case lon == 0 && lat == 0:
		return ""
	case lat < -60:
		return "" // Antarctica; Valve has no relays there, but do not guess.
	case lon >= -170 && lon < -30 && lat >= 12:
		return RegionNorthAmerica
	case lon >= -90 && lon < -30 && lat < 12:
		return RegionSouthAmerica
	case lon >= -30 && lon < 40 && lat < 35:
		return RegionAfrica
	case lon >= 34 && lon < 65 && lat >= 12 && lat < 42:
		return RegionMiddleEast
	case lon >= -30 && lon < 45 && lat >= 35:
		return RegionEurope
	case lon >= 110 && lat < -10:
		return RegionOceania
	case lon >= 130 && lat < 25:
		return RegionOceania
	default:
		return RegionAsia
	}
}

// haversineKm is the great-circle distance between two POPs, used to decide
// whether they are the same physical location.
func haversineKm(lon1, lat1, lon2, lat2 float64) float64 {
	const earthRadiusKm = 6371.0
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := rad(lat2 - lat1)
	dLon := rad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(a)))
}
