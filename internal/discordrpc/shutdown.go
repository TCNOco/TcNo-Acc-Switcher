package discordrpc

import "time"

const exitShutdownTimeout = 2 * time.Second

// StopForExit gives Discord a brief opportunity to clear presence without
// preventing application exit. rich-go's handshake and SET_ACTIVITY calls read
// from IPC without a deadline; either Stop's locks or the final clear can wait
// forever. Keep all access to rich-go serialized by Stop, including after the
// timeout: calling Logout concurrently with a pending read would race its socket.
// This is only for process exit, not for disabling/restarting RPC in a live app.
func (m *Manager) StopForExit() {
	logRPC().Info("stopping Discord RPC for application exit")
	if !waitForExitStop(m.Stop, exitShutdownTimeout) {
		logRPC().Warn("Discord shutdown timed out; continuing application exit")
		return
	}
	logRPC().Info("Discord RPC shutdown complete")
}

func waitForExitStop(stop func(), timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		stop()
		close(done)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}
