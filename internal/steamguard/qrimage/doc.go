// Package qrimage loads bounded screenshot images and extracts canonical Steam
// mobile-login QR challenges. It does not authorize a decoded challenge.
//
// The decoder must fit the module grid under perspective: inputs include
// photographs of a screen, and an affine-only fit locates the finder patterns
// and then fails Reed-Solomon.
//
// The gozxing reader is constructed per decode rather than shared: the upstream
// thread-safety work is not tagged for release. The package-level state in
// v0.1.1 is specification lookup tables that are not written after init.
//
// The decoder returns only the first symbol in an image and offers no in-call
// cancellation hook. DecodeCandidates therefore scans a bounded set of
// overlapping, zero-copy regions, retains at most two distinct canonical
// challenges (enough to reject ambiguity), checks cancellation and elapsed time
// around every synchronous decode, and consumes and wipes every input frame.
// The frames this package owns are wiped; the decoder's own working copies of
// the image are not reachable to wipe.
package qrimage
