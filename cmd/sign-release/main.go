package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/ssh"
)

func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	switch len(data) {
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(data), nil
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(data), nil
	}

	signer, err := ssh.ParseRawPrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}
	key, ok := signer.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is %T, want ed25519", signer)
	}
	return key, nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: sign-release <private-key-file> <artifact-file>\n")
		os.Exit(2)
	}

	key, err := loadPrivateKey(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading key: %v\n", err)
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
	sig := ed25519.Sign(key, h.Sum(nil))
	fmt.Println(base64.StdEncoding.EncodeToString(sig))
}
