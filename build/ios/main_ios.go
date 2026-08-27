//go:build ios

package main

import (
	"C"
)

// For iOS builds, we need to export a function that can be called from Objective-C
// This wrapper allows us to keep the original main.go unmodified

//export WailsIOSMain
func WailsIOSMain() {
	// DO NOT lock the goroutine to the current OS thread on iOS!
	// This causes signal handling issues:
	// "signal 16 received on thread with no signal stack"
	// "fatal error: non-Go code disabled sigaltstack"

	main()
}