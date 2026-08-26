// Package qrimage loads bounded screenshot images and extracts canonical Steam
// mobile-login QR challenges. It does not authorize a decoded challenge.
//
// The decoder must fit the module grid under perspective: inputs include
// photographs of a screen, and an affine-only fit locates the finder patterns
// and then fails Reed-Solomon.
//
// Decoder audit (2026-08-24): github.com/makiuchi-d/gozxing v0.1.1 is a pure Go
// port of ZXing, MIT-licensed, with no cgo and no assembly. Its production
// import closure for qrcode adds nothing new to this module graph: golang.org/x
// /text was already a direct dependency and golang.org/x/xerrors was already
// present as an indirect one. deps.dev reported no advisories for the exact
// version, and the repository is active and unarchived. The thread-safety work
// upstream is not tagged for release, so the reader is constructed per decode
// rather than shared; the package-level state in v0.1.1 is specification lookup
// tables that are not written after init.
//
// The decoder returns only the first symbol in an image and offers no in-call
// cancellation hook. DecodeCandidates therefore scans a bounded set of
// overlapping, zero-copy regions, retains at most two distinct canonical
// challenges (enough to reject ambiguity), checks cancellation and elapsed time
// around every synchronous decode, and consumes and wipes every input frame.
// The frames this package owns are wiped; the decoder's own working copies of
// the image are not reachable to wipe.
package qrimage
