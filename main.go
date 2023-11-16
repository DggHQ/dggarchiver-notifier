package main

import (
	"log/slog"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	"github.com/DggHQ/dggarchiver-notifier/platforms"
	"github.com/DggHQ/dggarchiver-notifier/state"
)

func main() {
	cfg := config.New()

	state := state.New(cfg)
	state.Load()

	slog.Info("running the notifier service")

	p := platforms.New(cfg, &state)
	p.Start()
}
