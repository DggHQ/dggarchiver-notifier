package main

import (
	"reflect"
	"sync"
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	log "github.com/DggHQ/dggarchiver-logger"
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
	cfg.Load()

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

	var numOfEnabledPlatforms int64
	var platformPrioritySum int
	platformsValue := reflect.ValueOf(cfg.Notifier.Platforms)
	platformsFields := reflect.VisibleFields(reflect.TypeOf(cfg.Notifier.Platforms))
	for _, field := range platformsFields {
		if platformsValue.FieldByName(field.Name).FieldByName("Enabled").Bool() {
			numOfEnabledPlatforms++
			platformPrioritySum = platformPrioritySum + int(platformsValue.FieldByName(field.Name).FieldByName("Priority").Int())
		}
	}

	var i int64

	for i = 1; i < numOfEnabledPlatforms+1; i++ {
		if platformsValue.FieldByName("YouTube").FieldByName("Priority").Int() == i || platformsValue.FieldByName("YouTube").FieldByName("Priority").Int() == 0 {
			if cfg.Notifier.Platforms.YouTube.Enabled {
				if cfg.Notifier.Platforms.YouTube.APIRefresh != 0 {
					log.Infof("Checking YT API every %d minute(s)", cfg.Notifier.Platforms.YouTube.APIRefresh)
					sleepTime := time.Second * 60 * time.Duration(cfg.Notifier.Platforms.YouTube.APIRefresh)
					wg.Add(1)
					yt.StartYTThread("[YT] [API]", yt.LoopApiLivestream, &cfg, &state, sleepTime)
				}

				if cfg.Notifier.Platforms.YouTube.ScraperRefresh != 0 {
					log.Infof("Checking YT scraped page every %d minute(s)", cfg.Notifier.Platforms.YouTube.ScraperRefresh)
					sleepTime := time.Second * 60 * time.Duration(cfg.Notifier.Platforms.YouTube.ScraperRefresh)
					wg.Add(1)
					yt.StartYTThread("[YT] [SCRAPER]", yt.LoopScrapedLivestream, &cfg, &state, sleepTime)
				}
			}
			time.Sleep(1 * time.Second)
		}
		if platformsValue.FieldByName("Rumble").FieldByName("Priority").Int() == i || platformsValue.FieldByName("Rumble").FieldByName("Priority").Int() == 0 {
			if cfg.Notifier.Platforms.Rumble.Enabled {
				if cfg.Notifier.Platforms.Rumble.ScraperRefresh != 0 {
					log.Infof("Checking Rumble scraped page every %d minute(s)", cfg.Notifier.Platforms.Rumble.ScraperRefresh)
					sleepTime := time.Second * 60 * time.Duration(cfg.Notifier.Platforms.Rumble.ScraperRefresh)
					wg.Add(1)
					rumble.StartRumbleThread("[Rumble] [SCRAPER]", rumble.LoopScrapedLivestream, &cfg, &state, sleepTime)
				}
			}
			time.Sleep(1 * time.Second)
		}
		if platformsValue.FieldByName("Kick").FieldByName("Priority").Int() == i || platformsValue.FieldByName("Kick").FieldByName("Priority").Int() == 0 {
			if cfg.Notifier.Platforms.Kick.Enabled {
				if cfg.Notifier.Platforms.Kick.ScraperRefresh != 0 {
					log.Infof("Checking Kick scraped API every %d minute(s)", cfg.Notifier.Platforms.Kick.ScraperRefresh)
					sleepTime := time.Second * 60 * time.Duration(cfg.Notifier.Platforms.Kick.ScraperRefresh)
					kick.InitializeKickScraper(&cfg)
					wg.Add(1)
					kick.StartKickThread("[Kick] [SCRAPER]", kick.LoopScrapedLivestream, &cfg, &state, sleepTime)
				}
			}
			time.Sleep(1 * time.Second)
		}
		if platformPrioritySum == 0 {
			break
		}
	}

	wg.Wait()
}
