package main

import (
	"sync"
	"time"

	log "github.com/DggHQ/dggarchiver-logger"
	"github.com/DggHQ/dggarchiver-notifier/config"
	"github.com/DggHQ/dggarchiver-notifier/platforms/kick"
	"github.com/DggHQ/dggarchiver-notifier/platforms/rumble"
	"github.com/DggHQ/dggarchiver-notifier/platforms/yt"
	"github.com/DggHQ/dggarchiver-notifier/util"
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
	cfg.Initialize()

	if cfg.Notifier.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	state := util.State{
		SearchETag: "",
		SentVODs:   make([]string, 0),
	}
	state.Load()

	var wg sync.WaitGroup
	log.Infof("Running the notifier service in continuous mode...")

	if cfg.Notifier.Platforms.YTConfig.Enabled {
		if cfg.Notifier.Platforms.YTConfig.APIRefresh != 0 {
			log.Infof("Checking YT API every %d minute(s)", cfg.Notifier.Platforms.YTConfig.APIRefresh)
			sleepTime := time.Second * 60 * time.Duration(cfg.Notifier.Platforms.YTConfig.APIRefresh)
			wg.Add(1)
			yt.StartYTThread("[YT] [API]", yt.LoopApiLivestream, &cfg, &state, sleepTime)
		}

		if cfg.Notifier.Platforms.YTConfig.ScraperRefresh != 0 {
			log.Infof("Checking YT scraped page every %d minute(s)", cfg.Notifier.Platforms.YTConfig.ScraperRefresh)
			sleepTime := time.Second * 60 * time.Duration(cfg.Notifier.Platforms.YTConfig.ScraperRefresh)
			wg.Add(1)
			yt.StartYTThread("[YT] [SCRAPER]", yt.LoopScrapedLivestream, &cfg, &state, sleepTime)
		}
	}

	time.Sleep(1 * time.Second)

	if cfg.Notifier.Platforms.RumbleConfig.Enabled {
		if cfg.Notifier.Platforms.RumbleConfig.ScraperRefresh != 0 {
			log.Infof("Checking Rumble scraped page every %d minute(s)", cfg.Notifier.Platforms.RumbleConfig.ScraperRefresh)
			sleepTime := time.Second * 60 * time.Duration(cfg.Notifier.Platforms.RumbleConfig.ScraperRefresh)
			wg.Add(1)
			rumble.StartRumbleThread("[Rumble] [SCRAPER]", rumble.LoopScrapedLivestream, &cfg, &state, sleepTime)
		}
	}

	time.Sleep(1 * time.Second)

	if cfg.Notifier.Platforms.KickConfig.Enabled {
		if cfg.Notifier.Platforms.KickConfig.ScraperRefresh != 0 {
			log.Infof("Checking Kick scraped API every %d minute(s)", cfg.Notifier.Platforms.KickConfig.ScraperRefresh)
			sleepTime := time.Second * 60 * time.Duration(cfg.Notifier.Platforms.KickConfig.ScraperRefresh)
			wg.Add(1)
			kick.StartKickThread("[Kick] [SCRAPER]", kick.LoopScrapedLivestream, &cfg, &state, sleepTime)
		}
	}

	wg.Wait()
}
