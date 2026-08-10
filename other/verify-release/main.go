// Verifies a release signature the same way the shipped app will:
// normalize updater-key.pub, parse it like the Wails updater, and check the
// ed25519 signature over sha256(artifact). Run in CI right after signing so
// a key/secret mismatch fails the build instead of every user's update.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"strings"

	"TcNo-Acc-Switcher/internal/updatecheck"
)

func loadPublicKey(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw := updatecheck.NormalizePublicKey(data)
	// Mirror wails/v3 pkg/updater parseEd25519Public: raw, then PEM/PKIX.
	if len(raw) == ed25519.PublicKeySize {
		return ed25519.PublicKey(raw), nil
	}
	if block, _ := pem.Decode(raw); block != nil {
		raw = block.Bytes
	}
	pubAny, err := x509.ParsePKIXPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("public key is not raw, PEM or PKIX ed25519: %w", err)
	}
	pub, ok := pubAny.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is %T, want ed25519", pubAny)
	}
	return pub, nil
}

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "Usage: verify-release <public-key-file> <artifact-file> <signature-file>\n")
		os.Exit(2)
	}

	pub, err := loadPublicKey(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading public key: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Open(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "opening artifact: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		fmt.Fprintf(os.Stderr, "hashing: %v\n", err)
		os.Exit(1)
	}

	sigB64, err := os.ReadFile(os.Args[3])
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading signature: %v\n", err)
		os.Exit(1)
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigB64)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "decoding signature: %v\n", err)
		os.Exit(1)
	}

	if !ed25519.Verify(pub, h.Sum(nil), sig) {
		fmt.Fprintln(os.Stderr, "FAIL: signature does not verify against the public key")
		os.Exit(1)
	}
	fmt.Println("Signature verifies against the embedded updater public key.")
}
