package serverpicker

import (
	"context"
	"time"
)

const (
	// probeTimeout is generous relative to discoverTimeout: by then we have
	// committed to one relay and want a real loss figure, not a fast one.
	discoverTimeout = 800 * time.Millisecond
	probeTimeout    = 1000 * time.Millisecond
	probeCount      = 4
	// maxDiscoverRelays caps phase one. POPs advertise up to 14 relays and they
	// sit in the same rack, so probing every one buys nothing but wall time.
	maxDiscoverRelays = 4
)

// PingResult is one POP's measurement. Reachable is false when nothing answered,
// in which case RTT and Loss carry no meaning.
type PingResult struct {
	PopID     string  `json:"popId"`
	Reachable bool    `json:"reachable"`
	RTTms     int     `json:"rttMs"`
	Loss      float64 `json:"loss"`
}

// measurePOP finds the POP's fastest relay, then probes that one repeatedly for
// a loss figure. Splitting the two phases keeps the loss number attributable to
// a single host rather than smeared across relays with different paths.
func measurePOP(ctx context.Context, pop POP) PingResult {
	res := PingResult{PopID: pop.ID}

	best := ""
	bestRTT := -1
	for i, ip := range pop.Relay {
		if i >= maxDiscoverRelays {
			break
		}
		if ctx.Err() != nil {
			return res
		}
		rtt, ok := icmpEcho(ip, discoverTimeout)
		if ok && (bestRTT < 0 || rtt < bestRTT) {
			best, bestRTT = ip, rtt
		}
	}
	if best == "" {
		return res
	}

	successes := 0
	minRTT := bestRTT
	for i := 0; i < probeCount; i++ {
		if ctx.Err() != nil {
			break
		}
		rtt, ok := icmpEcho(best, probeTimeout)
		if !ok {
			continue
		}
		successes++
		if rtt < minRTT {
			minRTT = rtt
		}
	}
	if successes == 0 {
		// Phase one reached it, so report that rather than claiming unreachable.
		res.Reachable = true
		res.RTTms = bestRTT
		res.Loss = 100
		return res
	}

	res.Reachable = true
	res.RTTms = minRTT
	res.Loss = lossPercent(successes, probeCount)
	return res
}

// lossPercent is the share of probes that went unanswered. Float division on
// purpose: integer division here collapses every partial loss to 0 % or 100 %.
func lossPercent(successes, probes int) float64 {
	if probes <= 0 {
		return 0
	}
	if successes < 0 {
		successes = 0
	}
	if successes > probes {
		successes = probes
	}
	return (1 - float64(successes)/float64(probes)) * 100
}
