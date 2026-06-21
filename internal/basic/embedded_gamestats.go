package basic

var embeddedGameStatsJSON []byte

// SetEmbeddedGameStatsJSON is called from main (single //go:embed at module root).
func SetEmbeddedGameStatsJSON(b []byte) {
	embeddedGameStatsJSON = append([]byte(nil), b...)
}

func embeddedGameStatsBytes() []byte {
	return embeddedGameStatsJSON
}
