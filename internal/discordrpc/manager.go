package discordrpc

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/stats"

	richgo "github.com/hugolgst/rich-go/client"
)

const (
	clientID             = "973188269405765682"
	refreshPeriod        = 30 * time.Second
	discordLargeImageKey = "switcher"
	discordSmallImageKey = "switcher_small"

	// connectRetryMin and connectRetryMax bound the wait after a failed connect.
	//
	// Connecting to a Discord that is not running is not cheap: the IPC dial
	// retries a named pipe for a full two seconds before giving up, and neither
	// rich-go nor this manager remembers that it failed, so every refresh paid
	// it again. Backing off keeps an absent Discord from costing two seconds out
	// of every thirty for the whole session, while still picking presence up
	// within a few minutes of Discord starting.
	connectRetryMin = 30 * time.Second
	connectRetryMax = 5 * time.Minute
)

// discordLogin is richgo.Login behind a variable so tests can substitute a slow
// or failing connect without talking to a real Discord.
var discordLogin = richgo.Login

type Manager struct {
	mu        sync.Mutex
	refreshMu sync.Mutex

	initialized bool
	startedAt   time.Time
	stopCh      chan struct{}
	stopping    bool

	connectBackoff      time.Duration
	connectBackoffUntil time.Time

	lastDetails string
	lastState   string
}

func logRPC() *slog.Logger {
	return slog.Default().With("component", "discord-rpc")
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Start() {
	m.mu.Lock()
	if m.stopCh != nil {
		m.mu.Unlock()
		logRPC().Debug("start skipped: manager already running")
		return
	}
	m.stopCh = make(chan struct{})
	m.stopping = false
	stopCh := m.stopCh
	m.mu.Unlock()

	logRPC().Info("manager started", "refreshPeriod", refreshPeriod.String())
	go m.runPeriodic(stopCh)
	// Asynchronous deliberately. Start runs before the window exists, and the
	// first refresh connects to Discord - a dial that takes two seconds to fail
	// when Discord is not running. Nothing on screen waits for presence.
	m.RefreshAsync()
}

func (m *Manager) Stop() {
	// Signal before taking refreshMu, so a refresh that has not reached the
	// connect yet bails instead of making shutdown wait out its dial.
	m.mu.Lock()
	m.stopping = true
	stopCh := m.stopCh
	m.stopCh = nil
	m.mu.Unlock()
	if stopCh != nil {
		close(stopCh)
	}

	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	logRPC().Info("manager stopping")
	m.shutdown()
}

func (m *Manager) isStopping() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopping
}

func (m *Manager) RefreshAsync() {
	go m.Refresh()
}

func (m *Manager) Refresh() {
	if m.isStopping() {
		return
	}
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	if m.isStopping() {
		return
	}

	settings, err := loadCurrentSettings()
	if err != nil {
		logRPC().Warn("refresh skipped: failed to load settings", "err", err)
		return
	}
	if settings.OfflineMode || !settings.DiscordRpc {
		logRPC().Info("refresh gate: rpc disabled", "offlineMode", settings.OfflineMode, "discordRpc", settings.DiscordRpc)
		m.shutdown()
		return
	}
	if err := m.ensureStarted(); err != nil {
		logRPC().Warn("refresh skipped: rpc start failed", "err", err)
		return
	}

	activity := richgo.Activity{
		State:      "",
		Details:    "Currently switching accounts",
		LargeImage: discordLargeImageKey,
		LargeText:  "TcNo Account Switcher",
		SmallImage: discordSmallImageKey,
		SmallText:  "TcNo Account Switcher",
		Buttons: []*richgo.Button{
			{Label: "Website", Url: "https://github.com/TCNOCo/TcNo-Acc-Switcher/"},
		},
		Timestamps: &richgo.Timestamps{Start: &m.startedAt},
	}

	if settings.StatsEnabled && settings.DiscordRpcShare {
		if report, err := stats.GetReportData(); err == nil {
			activity.State = fmt.Sprintf("Accounts Switched: %d", report.TotalSwitches)
		} else {
			logRPC().Warn("stats unavailable for rpc share state", "err", err)
		}
	}

	if activity.Details == m.lastDetails && activity.State == m.lastState {
		if err := richgo.SetActivity(activity); err != nil {
			logRPC().Warn("set activity failed", "err", err)
		}
		return
	}

	if err := richgo.SetActivity(activity); err != nil {
		logRPC().Warn("set activity failed", "err", err)
		return
	}
	m.lastDetails = activity.Details
	m.lastState = activity.State
	logRPC().Debug("activity updated", "details", activity.Details, "state", activity.State)
}

func (m *Manager) ensureStarted() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.initialized {
		return nil
	}
	if now := time.Now(); now.Before(m.connectBackoffUntil) {
		return fmt.Errorf("discord connect backing off for another %s",
			m.connectBackoffUntil.Sub(now).Round(time.Second))
	}
	if err := discordLogin(clientID); err != nil {
		m.noteConnectFailedLocked()
		return err
	}
	m.connectBackoff = 0
	m.connectBackoffUntil = time.Time{}
	now := time.Now()
	m.startedAt = now
	m.initialized = true
	logRPC().Info("rpc client initialized", "clientID", clientID)
	return nil
}

// noteConnectFailedLocked doubles the wait before the next connect is attempted,
// up to connectRetryMax. Callers hold m.mu.
func (m *Manager) noteConnectFailedLocked() {
	switch {
	case m.connectBackoff <= 0:
		m.connectBackoff = connectRetryMin
	case m.connectBackoff < connectRetryMax:
		m.connectBackoff *= 2
	}
	m.connectBackoff = min(m.connectBackoff, connectRetryMax)
	m.connectBackoffUntil = time.Now().Add(m.connectBackoff)
	logRPC().Debug("discord connect failed; backing off", "retryIn", m.connectBackoff.String())
}

func (m *Manager) shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.initialized {
		return
	}
	m.lastDetails = ""
	m.lastState = ""
	if err := clearPresenceDiscord(); err != nil {
		logRPC().Warn("clear presence before logout failed", "err", err)
	} else {
		logRPC().Info("presence cleared (SET_ACTIVITY null)")
	}
	richgo.Logout()
	m.initialized = false
	m.startedAt = time.Time{}
	logRPC().Info("rpc client logged out")
}

func (m *Manager) runPeriodic(stopCh <-chan struct{}) {
	ticker := time.NewTicker(refreshPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			m.Refresh()
		}
	}
}

func loadCurrentSettings() (platform.AppSettings, error) {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return platform.AppSettings{}, err
	}
	settings, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return platform.AppSettings{}, err
	}
	settings.Language = strings.TrimSpace(settings.Language)
	return settings, nil
}
