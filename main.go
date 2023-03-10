package main

import (
	"sync"
	"time"

	log "github.com/DggHQ/dggarchiver-logger"
	"github.com/DggHQ/dggarchiver-notifier/config"
	"github.com/DggHQ/dggarchiver-notifier/util"
	"github.com/DggHQ/dggarchiver-notifier/yt"
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

	if cfg.Flags.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	state := util.State{
		SearchETag: "",
		SentVODs:   make([]string, 0),
	}
	state.Load()

	var wg sync.WaitGroup
	log.Infof("Running the application in continuous mode, checking YT scraped page every %d minute(s) and YT API every %d minute(s)", cfg.YTConfig.YTRefresh, cfg.YTConfig.YTAPIRefresh)

	if cfg.YTConfig.YTAPIRefresh != 0 {
		ytApiSleepTime := time.Second * 60 * time.Duration(cfg.YTConfig.YTAPIRefresh)
		wg.Add(1)
		yt.StartYTThread("[YT] [API]", yt.LoopApiLivestream, &cfg, &state, ytApiSleepTime)
	}

	if cfg.YTConfig.YTRefresh != 0 {
		ytSleepTime := time.Second * 60 * time.Duration(cfg.YTConfig.YTRefresh)
		wg.Add(1)
		yt.StartYTThread("[YT] [SCRAPER]", yt.LoopScrapedLivestream, &cfg, &state, ytSleepTime)
	}

	wg.Wait()
}
