package protocol

import (
	"math"
	"time"
)

const (
	defaultAuthPollInterval = 5 * time.Second
	minAuthPollInterval     = 250 * time.Millisecond
)

// Steam's optional interval is a scheduling hint, not an authentication result.
// Use a conservative default for zero/omitted values and a minimum delay for
// small positive values. Preserve longer server delays instead of polling early.
func authPollInterval(seconds float32) (time.Duration, bool) {
	value := float64(seconds)
	nanoseconds := value * float64(time.Second)
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || nanoseconds >= float64(math.MaxInt64) {
		return 0, false
	}
	if value == 0 {
		return defaultAuthPollInterval, true
	}
	interval := time.Duration(nanoseconds)
	if interval < minAuthPollInterval {
		interval = minAuthPollInterval
	}
	return interval, true
}
