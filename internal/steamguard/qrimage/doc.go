// Package qrimage loads bounded screenshot images and extracts canonical Steam
// mobile-login QR challenges. It does not authorize a decoded challenge.
//
// Decoder audit (2026-07-22): github.com/piglig/go-qr v1.1.0 is pure Go and
// MIT-licensed at tag 832517b8dd4c5f48188211c0c6691c6ac38b0363. Its production
// decoder closure imports only the Go standard library and its own internal
// Reed-Solomon package. Testify, spew, difflib, and yaml are upstream test-only
// requirements already present in this module graph. The tagged repository was
// active and unarchived, and an exact-version OSV query returned no advisories.
// Decode calls DecodeDetailed, whose contract is to return the first symbol.
// Its robust locator sorts finder candidates by confidence and keeps only the
// strongest three, so that API cannot enumerate multiple symbols. It also has no
// in-call cancellation hook. DecodeCandidates therefore scans a bounded set of
// overlapping, zero-copy regions, retains at most two distinct canonical
// challenges (enough to reject ambiguity), checks cancellation and elapsed time
// around every synchronous decode, and consumes and wipes every input frame.
package qrimage
