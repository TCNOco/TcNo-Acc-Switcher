package updatecheck

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizePublicKeyOpenSSH(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authorized := []byte("ssh-ed25519 " + base64.StdEncoding.EncodeToString(sshWireKey("ssh-ed25519", pub)) + " updater@tcno\n")

	got := NormalizePublicKey(authorized)
	if !bytes.Equal(got, pub) {
		t.Fatalf("NormalizePublicKey = %x, want raw key %x", got, []byte(pub))
	}
}

// authorized_keys shapes OpenSSH accepts but NormalizePublicKey deliberately
// does not: they must pass through untouched rather than yield a wrong key.
func TestNormalizePublicKeyRejectsUnsupportedLines(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	body := base64.StdEncoding.EncodeToString(sshWireKey("ssh-ed25519", pub))
	for name, line := range map[string]string{
		"comment line":  "# updater key\nssh-ed25519 " + body + "\n",
		"options":       `command="/bin/true",no-pty ssh-ed25519 ` + body + "\n",
		"wrong algo":    "ssh-rsa " + body + "\n",
		"truncated key": "ssh-ed25519 " + base64.StdEncoding.EncodeToString(sshWireKey("ssh-ed25519", pub[:16])) + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got := NormalizePublicKey([]byte(line)); !bytes.Equal(got, []byte(line)) {
				t.Fatalf("line was parsed as %x, want passthrough", got)
			}
		})
	}
}

func sshWireKey(algo string, key []byte) []byte {
	blob := binary.BigEndian.AppendUint32(nil, uint32(len(algo)))
	blob = append(blob, algo...)
	blob = binary.BigEndian.AppendUint32(blob, uint32(len(key)))
	return append(blob, key...)
}

func TestNormalizePublicKeyPassthrough(t *testing.T) {
	raw := make([]byte, ed25519.PublicKeySize)
	if got := NormalizePublicKey(raw); !bytes.Equal(got, raw) {
		t.Fatalf("raw key changed: %x", got)
	}
	garbage := []byte("not a key")
	if got := NormalizePublicKey(garbage); !bytes.Equal(got, garbage) {
		t.Fatalf("garbage changed: %x", got)
	}
}

// The embedded updater-key.pub must normalize to a raw ed25519 key, or the
// Wails updater rejects every signed release at verify time.
func TestEmbeddedUpdaterKeyNormalizes(t *testing.T) {
	got := NormalizePublicKey(readUpdaterKey(t))
	if len(got) != ed25519.PublicKeySize {
		t.Fatalf("normalized key is %d bytes, want %d; updater cannot use it", len(got), ed25519.PublicKeySize)
	}
}

// Known-answer vector: sha256 of the published v4.0.6 TcNo-Acc-Switcher.exe
// and its .exe.sig release asset. Proves updater-key.pub matches the CI
// signing secret (UPDATER_KEY), not just that it parses — if this fails
// after a deliberate key rotation, re-pin the vector from the first release
// signed with the new key.
func TestEmbeddedUpdaterKeyMatchesReleaseSignature(t *testing.T) {
	digest, err := hex.DecodeString("d3a4d5e063e4ecff7d76938fc7a0a4794802492cdec1b7c6a0f236b6e35f1d73")
	if err != nil {
		t.Fatal(err)
	}
	sig, err := base64.StdEncoding.DecodeString("/tEpODL7wUgSuJXhkPewe6FVOwgH21lneAvn4V7nbdL41/tFYDD6Ce2nNv0ZTyjFn74OdTcSF58Rm5dKcXpiCA==")
	if err != nil {
		t.Fatal(err)
	}

	key := NormalizePublicKey(readUpdaterKey(t))
	if len(key) != ed25519.PublicKeySize {
		t.Fatalf("normalized key is %d bytes, want %d", len(key), ed25519.PublicKeySize)
	}
	if !ed25519.Verify(ed25519.PublicKey(key), digest, sig) {
		t.Fatal("v4.0.6 release signature does not verify against updater-key.pub; released updates will fail to install")
	}
}

func readUpdaterKey(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "updater-key.pub"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
