package streamer

import "strings"

// broadcastExes are the image names that mean "what is on this screen is going out
// to an audience". Lowercase, matched against the process image base name.
//
// The list is deliberately narrow. Capture helpers that sit resident on millions of
// machines that never stream — NVIDIA Share / NVIDIA app, Radeon Software, Xbox Game
// Bar, ShareX, Stream Deck, Discord — are excluded on purpose: a streamer mode that
// is permanently on is the same as no streamer mode at all. Everything here is
// something a person opens when they are about to broadcast or record.
var broadcastExes = []string{
	// OBS Studio and the forks that keep its process name.
	"obs64.exe",
	"obs32.exe",
	"obs.exe",

	// Streamlabs Desktop (still ships under both names across versions).
	"streamlabs obs.exe",
	"streamlabs desktop.exe",

	// XSplit.
	"xsplit.core.exe",
	"xsplit.broadcaster.exe",
	"xsplitgamecaster.exe",

	// Other broadcasters.
	"twitch studio.exe",
	"prismlivestudio.exe",
	"vmix64.exe",
	"vmix.exe",
	"wirecast.exe",
	"castermuse.exe",
	"nvidia broadcast.exe",

	// Virtual camera / multi-source apps that are only open while presenting.
	"manycam.exe",
	"splitcam.exe",

	// Screen recorders — same exposure, same need.
	"bdcam.exe",
	"action.exe",
	"camtasiarecorder.exe",
	"camtasia.exe",
	"loom.exe",
	"screenrec.exe",
}

// targetSet builds the lookup the watcher hot path uses.
func targetSet() map[string]struct{} {
	out := make(map[string]struct{}, len(broadcastExes))
	for _, name := range broadcastExes {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			out[name] = struct{}{}
		}
	}
	return out
}
