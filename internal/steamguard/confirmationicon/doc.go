// Package confirmationicon fetches and sanitizes untrusted confirmation
// images without exposing their remote URLs to the WebView.
//
// Integration must provide an exact, reviewed hostname allowlist. Do not build
// that allowlist from confirmation data, accept wildcards, or pass remote URLs
// through to frontend code. Steam's confirmation response currently carries an
// icon field, but this repository has no pinned upstream contract for its CDN
// hosts, so the package intentionally does not guess a Steam hostname list.
package confirmationicon
