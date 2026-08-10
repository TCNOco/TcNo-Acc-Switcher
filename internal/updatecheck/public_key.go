package updatecheck

import (
	"crypto/ed25519"

	"golang.org/x/crypto/ssh"
)

// NormalizePublicKey converts an OpenSSH authorized_keys ed25519 public key
// (ssh-keygen's default output, as embedded from updater-key.pub) into the
// raw 32-byte form the Wails updater accepts. Raw, PEM and PKIX DER keys are
// returned unchanged for the updater to parse itself.
func NormalizePublicKey(key []byte) []byte {
	pub, _, _, _, err := ssh.ParseAuthorizedKey(key)
	if err != nil {
		return key
	}
	cpk, ok := pub.(ssh.CryptoPublicKey)
	if !ok {
		return key
	}
	if raw, ok := cpk.CryptoPublicKey().(ed25519.PublicKey); ok {
		return raw
	}
	return key
}
