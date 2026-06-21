//go:build windows

package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"TcNo-Acc-Switcher/internal/crashlog"

	"github.com/Microsoft/go-winio"
)

// PipePath is the Windows named pipe used for forwarding argv to the running GUI.
const PipePath = `\\.\pipe\TcNo-Acc-Switcher`

// ArgsEnvelope is one line-delimited JSON message to the GUI process.
type ArgsEnvelope struct {
	Argv []string `json:"argv"`
}

// ForwardArgs connects to the running instance and forwards raw command-line args.
func ForwardArgs(argv []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := winio.DialPipeContext(ctx, PipePath)
	if err != nil {
		return fmt.Errorf("pipe dial: %w", err)
	}
	defer conn.Close()

	enc := ArgsEnvelope{Argv: append([]string(nil), argv...)}
	b, err := json.Marshal(enc)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}
	_, err = conn.Write(b)
	return err
}

// StartGUIServer listens for forwarded argv until stop is called.
// Returns a function that can be called to stop the listener.
func StartGUIServer(handler func(argv []string)) (func(), error) {
	l, err := winio.ListenPipe(PipePath, nil)
	if err != nil {
		return nil, err
	}
	stopCh := make(chan struct{})
	go func() {
		defer crashlog.Capture()
		for {
			c, err := l.Accept()
			if err != nil {
				if isClosedPipeErr(err) {
					return
				}
				select {
				case <-stopCh:
					return
				default:
				}
				log.Printf("ipc accept: %v", err)
				continue
			}
			go handleConn(c, handler)
		}
	}()
	stop := func() {
		select {
		case <-stopCh:
			return
		default:
			close(stopCh)
		}
		_ = l.Close()
	}
	return stop, nil
}

func handleConn(conn net.Conn, handler func(argv []string)) {
	defer crashlog.Capture()
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		log.Printf("ipc read: %v", err)
		return
	}
	var env ArgsEnvelope
	if err := json.Unmarshal(line, &env); err != nil {
		log.Printf("ipc json: %v", err)
		return
	}
	if handler != nil {
		handler(env.Argv)
	}
}

func isClosedPipeErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, net.ErrClosed) || errors.Is(opErr.Err, io.EOF) {
			return true
		}
	}
	return false
}
