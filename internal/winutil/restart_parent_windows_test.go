//go:build windows

package winutil

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestRestartParentWaitsForExitAndBothLocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRestartParentLockHelper$", "--", "restart-lock-helper")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || strings.TrimSpace(line) != "ready" {
		t.Fatalf("helper startup: %q, %v", line, err)
	}

	args := []string{"+ea", " value with spaces "}
	done := make(chan error, 1)
	go func() {
		got, err := WaitForRestartParent(restartArgs(args, cmd.Process.Pid))
		if err == nil && !reflect.DeepEqual(got, args) {
			err = fmt.Errorf("forwarded args = %q, want %q", got, args)
		}
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("handoff returned while the old instance still held both locks: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := stdin.Write([]byte("exit\n")); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"app", "wails"} {
		name := windows.StringToUTF16Ptr(fmt.Sprintf("TcNo-Restart-Test-%d-%s", cmd.Process.Pid, suffix))
		h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
		if err == nil {
			windows.CloseHandle(h)
			t.Fatalf("%s lock still exists after handoff", suffix)
		}
		if err != windows.ERROR_FILE_NOT_FOUND {
			t.Fatalf("check %s lock: %v", suffix, err)
		}
	}
}

// This child deliberately leaves both mutex handles open until process exit,
// matching the lifetime of Wails' lock during a GUI restart.
func TestRestartParentLockHelper(t *testing.T) {
	if os.Args[len(os.Args)-1] != "restart-lock-helper" {
		return
	}
	for _, suffix := range []string{"app", "wails"} {
		name := windows.StringToUTF16Ptr(fmt.Sprintf("TcNo-Restart-Test-%d-%s", os.Getpid(), suffix))
		if _, err := windows.CreateMutex(nil, false, name); err != nil {
			os.Exit(2)
		}
	}
	fmt.Println("ready")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	os.Exit(0)
}

func TestRestartParentValidationAndTimeout(t *testing.T) {
	for _, pid := range []string{"", "0", "-1", "4294967296", "bad", strconv.Itoa(os.Getpid())} {
		if _, err := WaitForRestartParent([]string{restartParentPrefix + pid}); err == nil {
			t.Errorf("accepted invalid parent %q", pid)
		}
	}
	args := []string{"+ea"}
	if got, err := WaitForRestartParent(args); err != nil || !reflect.DeepEqual(got, args) {
		t.Fatalf("normal launch changed: %q, %v", got, err)
	}
	if err := waitForRestartProcess(uint32(os.Getpid()), time.Millisecond); err == nil {
		t.Fatal("waiting on a live parent must time out")
	}
}

func TestRestartArgumentQuoting(t *testing.T) {
	want := []string{"app.exe", "", " value with spaces ", `a"b`, `C:\path with spaces\`}
	got, err := windows.DecomposeCommandLine(joinArgsUTF16(want))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("command-line round trip = %q, %v; want %q", got, err, want)
	}
}

func TestRestartInvalidArgumentsKeepSingleton(t *testing.T) {
	old := singletonReleaser
	defer func() { singletonReleaser = old }()
	released := false
	RegisterSingletonReleaser(func() { released = true })
	if err := RestartElevated([]string{"invalid\x00argument"}); err == nil {
		t.Fatal("expected invalid UTF-16 argument to fail")
	}
	if released {
		t.Fatal("failed launch released the running instance's singleton")
	}
}
