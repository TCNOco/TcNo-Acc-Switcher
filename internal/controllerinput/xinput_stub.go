//go:build !windows

package controllerinput

func newStateReader() stateReader {
	return nil
}
