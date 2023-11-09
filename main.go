package main

import (
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	log "github.com/DggHQ/dggarchiver-logger"
	"github.com/DggHQ/dggarchiver-notifier/platforms"
	"github.com/DggHQ/dggarchiver-notifier/state"
)

func init() {
	loc, err := time.LoadLocation("UTC")
	if err != nil {
		log.Fatalf("%s", err)
	}
	time.Local = loc
}

func main() {
	cfg := config.Config{}
	cfg.Load()

	if cfg.Notifier.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	state := state.New(&cfg)
	state.Load()

	log.Infof("Running the notifier service in continuous mode...")

	p := platforms.New(&cfg, &state)
	p.Start()
}
