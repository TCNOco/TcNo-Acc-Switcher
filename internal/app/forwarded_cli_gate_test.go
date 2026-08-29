package app

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestForwardedCLIGateQueuesCopiedArgvFIFO(t *testing.T) {
	var got [][]string
	gate := newForwardedCLIGate(func(argv []string) {
		got = append(got, append([]string(nil), argv...))
	})

	first := []string{"--open-page", "first"}
	gate.Submit(first)
	first[1] = "mutated"
	gate.Submit([]string{"--open-page", "second"})

	if len(got) != 0 {
		t.Fatalf("dispatched before ready: %v", got)
	}
	gate.Ready()
	gate.Ready()

	want := [][]string{
		{"--open-page", "first"},
		{"--open-page", "second"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dispatches = %v, want %v", got, want)
	}
}

func TestForwardedCLIGateDispatchesAfterReady(t *testing.T) {
	var got [][]string
	gate := newForwardedCLIGate(func(argv []string) {
		got = append(got, append([]string(nil), argv...))
	})

	gate.Ready()
	gate.Submit([]string{"--help"})

	want := [][]string{{"--help"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dispatches = %v, want %v", got, want)
	}
}

func TestForwardedCLIGateSubmitDuringDrainStaysFIFO(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var got [][]string
	gate := newForwardedCLIGate(func(argv []string) {
		if argv[0] == "queued" {
			close(started)
			<-release
		}
		got = append(got, append([]string(nil), argv...))
	})
	gate.Submit([]string{"queued"})

	readyDone := make(chan struct{})
	go func() {
		gate.Ready()
		close(readyDone)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("queued dispatch did not start")
	}
	gate.Submit([]string{"during-drain"})
	close(release)

	select {
	case <-readyDone:
	case <-time.After(time.Second):
		t.Fatal("gate did not finish draining")
	}

	want := [][]string{{"queued"}, {"during-drain"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dispatches = %v, want %v", got, want)
	}
}

func TestForwardedCLIGateConcurrentSubmitAndReady(t *testing.T) {
	const count = 64
	var gotMu sync.Mutex
	got := make(map[string]int, count)
	gate := newForwardedCLIGate(func(argv []string) {
		gotMu.Lock()
		got[argv[0]]++
		gotMu.Unlock()
	})

	start := make(chan struct{})
	var submitters sync.WaitGroup
	for i := 0; i < count; i++ {
		submitters.Add(1)
		go func(id int) {
			defer submitters.Done()
			<-start
			gate.Submit([]string{fmt.Sprintf("request-%d", id)})
		}(i)
	}
	readyDone := make(chan struct{})
	go func() {
		<-start
		gate.Ready()
		close(readyDone)
	}()

	close(start)
	submitters.Wait()
	<-readyDone
	gate.Ready()

	gotMu.Lock()
	defer gotMu.Unlock()
	if len(got) != count {
		t.Fatalf("unique dispatches = %d, want %d", len(got), count)
	}
	for id, seen := range got {
		if seen != 1 {
			t.Errorf("%s dispatched %d times, want once", id, seen)
		}
	}
}
