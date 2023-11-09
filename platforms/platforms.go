package platforms

import (
	"reflect"
	"sync"
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	log "github.com/DggHQ/dggarchiver-logger"
	"github.com/DggHQ/dggarchiver-notifier/platforms/kick"
	"github.com/DggHQ/dggarchiver-notifier/platforms/rumble"
	"github.com/DggHQ/dggarchiver-notifier/platforms/yt"
	"github.com/DggHQ/dggarchiver-notifier/state"
	luaLibs "github.com/vadv/gopher-lua-libs"
	lua "github.com/yuin/gopher-lua"
)

type Platforms struct {
	enabled     int64
	prioritySum int
	value       reflect.Value
	fields      []reflect.StructField
	wg          sync.WaitGroup
	state       *state.State
	cfg         *config.Config
}

func New(cfg *config.Config, state *state.State) *Platforms {
	var numOfEnabledPlatforms int64
	var platformPrioritySum int
	platformsValue := reflect.ValueOf(cfg.Notifier.Platforms)
	platformsFields := reflect.VisibleFields(reflect.TypeOf(cfg.Notifier.Platforms))
	for _, field := range platformsFields {
		if platformsValue.FieldByName(field.Name).FieldByName("Enabled").Bool() {
			numOfEnabledPlatforms++
			platformPrioritySum += int(platformsValue.FieldByName(field.Name).FieldByName("Priority").Int())
		}
	}

	return &Platforms{
		enabled:     numOfEnabledPlatforms,
		prioritySum: platformPrioritySum,
		value:       platformsValue,
		fields:      platformsFields,
		wg:          sync.WaitGroup{},
		state:       state,
		cfg:         cfg,
	}
}

func (p *Platforms) Start() {
	var i int64

	for i = 1; i < p.enabled+1; i++ {
		if p.value.FieldByName("YouTube").FieldByName("Priority").Int() == i || p.value.FieldByName("YouTube").FieldByName("Priority").Int() == 0 {
			if p.cfg.Notifier.Platforms.YouTube.Enabled {
				if p.cfg.Notifier.Platforms.YouTube.APIRefresh != 0 {
					log.Infof("Checking YT API every %d minute(s)", p.cfg.Notifier.Platforms.YouTube.APIRefresh)
					ytAPI := yt.NewAPI(p.cfg, p.state)
					p.wg.Add(1)
					launchLoop(p.cfg, ytAPI)
				}

				if p.cfg.Notifier.Platforms.YouTube.ScraperRefresh != 0 {
					log.Infof("Checking YT scraped page every %d minute(s)", p.cfg.Notifier.Platforms.YouTube.ScraperRefresh)
					ytScraper := yt.NewScraper(p.cfg, p.state)
					p.wg.Add(1)
					launchLoop(p.cfg, ytScraper)
				}
			}

			time.Sleep(1 * time.Second)
		}

		if p.value.FieldByName("Rumble").FieldByName("Priority").Int() == i || p.value.FieldByName("Rumble").FieldByName("Priority").Int() == 0 {
			if p.cfg.Notifier.Platforms.Rumble.Enabled {
				if p.cfg.Notifier.Platforms.Rumble.ScraperRefresh != 0 {
					log.Infof("Checking Rumble scraped page every %d minute(s)", p.cfg.Notifier.Platforms.Rumble.ScraperRefresh)
					r := rumble.New(p.cfg, p.state)
					p.wg.Add(1)
					launchLoop(p.cfg, r)
				}
			}

			time.Sleep(1 * time.Second)
		}

		if p.value.FieldByName("Kick").FieldByName("Priority").Int() == i || p.value.FieldByName("Kick").FieldByName("Priority").Int() == 0 {
			if p.cfg.Notifier.Platforms.Kick.Enabled {
				if p.cfg.Notifier.Platforms.Kick.ScraperRefresh != 0 {
					log.Infof("Checking Kick scraped API every %d minute(s)", p.cfg.Notifier.Platforms.Kick.ScraperRefresh)
					k := kick.New(p.cfg, p.state)
					p.wg.Add(1)
					launchLoop(p.cfg, k)
				}
			}

			time.Sleep(1 * time.Second)
		}

		if p.prioritySum == 0 {
			break
		}
	}

	p.wg.Wait()
}

type implementation interface {
	CheckLivestream(*lua.LState) error
	GetPrefix() string
	GetSleepTime() time.Duration
}

func launchLoop(cfg *config.Config, imp implementation) {
	prefix := imp.GetPrefix()
	sleep := imp.GetSleepTime()

	go func() {
		l := lua.NewState()
		defer l.Close()
		if cfg.Notifier.Plugins.Enabled {
			luaLibs.Preload(l)
			if err := l.DoFile(cfg.Notifier.Plugins.PathToPlugin); err != nil {
				log.Fatalf("Wasn't able to load the Lua script: %s", err)
			}
		}

		timeout := 0

		for {
			if timeout > 0 {
				log.Infof("%s Sleeping for %d seconds before starting...", prefix, timeout)
				time.Sleep(time.Second * time.Duration(timeout))
			}
			err := imp.CheckLivestream(l)
			if err != nil {
				log.Errorf("%s Got an error, will restart the loop: %v", prefix, err)
				switch {
				case timeout == 0:
					timeout = 1
				case (timeout >= 1 && timeout <= 32):
					timeout *= 2
				}
				continue
			}
			timeout = 0
			log.Infof("%s Sleeping for %.f minutes...", prefix, sleep.Minutes())
			time.Sleep(sleep)
		}
	}()
}
