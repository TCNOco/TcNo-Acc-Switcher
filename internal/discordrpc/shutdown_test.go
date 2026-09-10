package discordrpc

import (
	"testing"
	"time"
)

func TestExitStopWaitsForSuccessfulCleanup(t *testing.T) {
	cleaned := make(chan struct{})
	if !waitForExitStop(func() { close(cleaned) }, time.Second) {
		t.Fatal("normal cleanup timed out")
	}
	select {
	case <-cleaned:
	default:
		t.Fatal("exit continued before cleanup completed")
	}
}

func TestExitStopDoesNotWaitForUnresponsiveDiscord(t *testing.T) {
	release := make(chan struct{})
	finished := make(chan struct{})
	defer func() {
		close(release)
		<-finished
	}()
	result := make(chan bool, 1)
	go func() {
		result <- waitForExitStop(func() {
			<-release // A stalled IPC read or a lock held by that read.
			close(finished)
		}, 20*time.Millisecond)
	}()
	select {
	case completed := <-result:
		if completed {
			t.Fatal("blocked cleanup reported success")
		}
	case <-time.After(time.Second):
		t.Fatal("application exit is still blocked on Discord")
	}
}
