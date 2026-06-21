package basic

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	defaultGameStatTTL        = 3 * time.Hour
	defaultGameRunningStatTTL = 30 * time.Minute
)

// gameStatTTL is the per-game cache lifetime before stats are refreshed in the background.
type gameStatTTL time.Duration

func (t gameStatTTL) duration() time.Duration {
	d := time.Duration(t)
	if d <= 0 {
		return defaultGameStatTTL
	}
	return d
}

func (t gameStatTTL) runningDuration() time.Duration {
	d := time.Duration(t)
	if d <= 0 {
		return defaultGameRunningStatTTL
	}
	return d
}

func gameStatEffectiveTTL(def gameDefinition, accountID, liveAccountID string) time.Duration {
	ttl := def.TTL.duration()
	proc := strings.TrimSpace(def.ProcessName)
	if proc == "" {
		return ttl
	}
	liveAccountID = strings.TrimSpace(liveAccountID)
	accountID = strings.TrimSpace(accountID)
	if liveAccountID == "" || !strings.EqualFold(liveAccountID, accountID) {
		return ttl
	}
	if !isGameProcessRunning(proc) {
		return ttl
	}
	return def.GameRunningTTL.runningDuration()
}

func (t *gameStatTTL) UnmarshalJSON(data []byte) error {
	data = bytesTrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*t = 0
		return nil
	}
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*t = gameStatTTL(time.Duration(n) * time.Second)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("TTL: expected duration string or seconds number: %w", err)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		*t = 0
		return nil
	}
	if secs, err := strconv.ParseFloat(s, 64); err == nil {
		*t = gameStatTTL(time.Duration(secs) * time.Second)
		return nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("TTL: %w", err)
	}
	if d <= 0 {
		*t = 0
		return nil
	}
	*t = gameStatTTL(d)
	return nil
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func gameStatRowExpired(row userGameStat, ttl time.Duration) bool {
	if row.LastUpdated.IsZero() {
		return true
	}
	if ttl <= 0 {
		ttl = defaultGameStatTTL
	}
	return time.Since(row.LastUpdated) >= ttl
}
