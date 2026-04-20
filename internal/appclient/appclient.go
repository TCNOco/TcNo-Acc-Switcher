// Package appclient holds shared outbound HTTP infrastructure for all platforms.
package appclient

import (
	"net/http"
	"time"
)

// Shared is the process-wide HTTP client for profile downloads, community/API
// requests, and other outbound calls. Platform packages (Steam, Discord, …)
// should use this instead of constructing separate clients.
var Shared = &http.Client{
	Timeout: 30 * time.Second,
}
