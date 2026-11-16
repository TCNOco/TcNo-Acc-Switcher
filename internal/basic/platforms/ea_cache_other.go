//go:build !windows

package platforms

import "os"

func readCacheFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
