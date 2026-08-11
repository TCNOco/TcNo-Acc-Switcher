//go:build !windows

package steambrowser

// macOS and Linux backends are not written yet. The seam is deliberate: only
// this file and its Windows counterpart are platform specific, so adding one
// means implementing View over WKWebView or WebKitGTK and nothing else. See the
// mapping on the View interface for the native call each method corresponds to.
func newView(ViewOptions) (View, error) {
	return nil, ErrUnsupportedPlatform
}

// Supported reports whether this build can open session windows.
func Supported() bool { return false }
