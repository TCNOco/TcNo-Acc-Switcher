package platform

var embeddedPlatformsJSON []byte

// SetEmbeddedPlatformsJSON is called from main (single //go:embed at module root).
func SetEmbeddedPlatformsJSON(b []byte) {
	embeddedPlatformsJSON = append([]byte(nil), b...)
}
