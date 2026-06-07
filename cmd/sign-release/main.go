package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: sign-release <private-key-file> <artifact-file>\n")
		os.Exit(2)
	}

	priv, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading key: %v\n", err)
		os.Exit(1)
	}
	key := ed25519.PrivateKey(priv)

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
	sig := ed25519.Sign(key, h.Sum(nil))
	fmt.Println(base64.StdEncoding.EncodeToString(sig))
}
