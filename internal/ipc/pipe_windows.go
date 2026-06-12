//go:build windows

package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync/atomic"
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

// StartGUIServer listens for forwarded argv until the process exits.
// Returns a function that can be called to stop the listener.
func StartGUIServer(handler func(argv []string)) (func(), error) {
	l, err := winio.ListenPipe(PipePath, nil)
	if err != nil {
		return nil, err
	}
	var closed int32
	go func() {
		defer crashlog.Capture()
		for {
			c, err := l.Accept()
			if err != nil {
				if atomic.LoadInt32(&closed) != 0 {
					return
				}
				log.Printf("ipc accept: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			go func(conn net.Conn) {
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
			}(c)
		}
	}()
	stop := func() {
		atomic.StoreInt32(&closed, 1)
		_ = l.Close()
	}
	return stop, nil
}
