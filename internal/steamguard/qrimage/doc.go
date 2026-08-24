// Package qrimage loads bounded screenshot images and extracts canonical Steam
// mobile-login QR challenges. It does not authorize a decoded challenge.
//
// Decoder audit (2026-08-24): github.com/makiuchi-d/gozxing v0.1.1 is a pure Go
// port of ZXing, MIT-licensed, with no cgo and no assembly. Its production
// import closure for qrcode adds nothing new to this module graph: golang.org/x
// /text was already a direct dependency and golang.org/x/xerrors was already
// present as an indirect one. deps.dev reported no advisories for the exact
// version, and the repository is active and unarchived. A v0.1.2 with
// thread-safety work exists upstream but is not tagged for release; the reader
// is therefore constructed per decode rather than shared, and the package-level
// state in v0.1.1 is specification lookup tables that are not written after
// init.
//
// It replaced github.com/piglig/go-qr, which could not read a photograph of a
// screen. That decoder located the finder patterns and then failed
// Reed-Solomon, because it fits the module grid with an affine transform and a
// photograph is not an affine view of a flat surface; roughly ninety
// combinations of threshold, adaptive threshold, blur, rescale and small
// rotation all failed on one such image, and ZXing read it unaided. See
// TestDecodeCandidatesReadsAPhotographedCode.
//
// Both decoders return only the first symbol in an image and neither offers an
// in-call cancellation hook. DecodeCandidates therefore scans a bounded set of
// overlapping, zero-copy regions, retains at most two distinct canonical
// challenges (enough to reject ambiguity), checks cancellation and elapsed time
// around every synchronous decode, and consumes and wipes every input frame.
// The frames this package owns are wiped; the decoder's own working copies of
// the image are not reachable to wipe, which is true of any decoder that is not
// written here.
package qrimage
