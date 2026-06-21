package main

import (
	"TcNo-Acc-Switcher/internal/basic"

	_ "embed"
)

//go:embed GameStats.json
var embeddedGameStatsJSON []byte

func init() {
	basic.SetEmbeddedGameStatsJSON(embeddedGameStatsJSON)
}
