package updatecheck

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
)

const sshEd25519 = "ssh-ed25519"

// NormalizePublicKey converts an OpenSSH authorized_keys ed25519 public key
// (ssh-keygen's default output, as embedded from updater-key.pub) into the
// raw 32-byte form the Wails updater accepts. Raw, PEM and PKIX DER keys are
// returned unchanged for the updater to parse itself.
//
// The only accepted shape is a bare "ssh-ed25519 <base64> [comment]" line.
// Leading '#' comment lines and option prefixes (`no-pty ssh-ed25519 ...`),
// which authorized_keys allows, are not parsed here: a rotated
// updater-key.pub carrying either would fall through to the updater as an
// unusable key and silently break update verification.
func NormalizePublicKey(key []byte) []byte {
	fields := bytes.Fields(key)
	if len(fields) < 2 || string(fields[0]) != sshEd25519 {
		return key
	}
	blob, err := base64.StdEncoding.DecodeString(string(fields[1]))
	if err != nil {
		return key
	}
	const algLen = len(sshEd25519)
	const want = 4 + algLen + 4 + ed25519.PublicKeySize
	if len(blob) != want ||
		binary.BigEndian.Uint32(blob) != uint32(algLen) ||
		string(blob[4:4+algLen]) != sshEd25519 ||
		binary.BigEndian.Uint32(blob[4+algLen:]) != ed25519.PublicKeySize {
		return key
	}
	return blob[want-ed25519.PublicKeySize:]
}
