package platforms

import (
	"log/slog"
	"os"
	"reflect"
	"sync"
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
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
	platformsValue := reflect.ValueOf(cfg.Platforms)
	platformsFields := reflect.VisibleFields(reflect.TypeOf(cfg.Platforms))
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
			if p.cfg.Platforms.YouTube.Enabled {
				if p.cfg.Platforms.YouTube.APIRefresh != 0 {
					ytAPI := yt.NewAPI(p.cfg, p.state)
					slog.Info("running platform loop",
						ytAPI.GetPrefix(),
						slog.Int("refresh", p.cfg.Platforms.YouTube.APIRefresh),
					)
					p.wg.Add(1)
					launchLoop(p.cfg, ytAPI)
				}

				if p.cfg.Platforms.YouTube.ScraperRefresh != 0 {
					ytScraper := yt.NewScraper(p.cfg, p.state)
					slog.Info("running platform loop",
						ytScraper.GetPrefix(),
						slog.Int("refresh", p.cfg.Platforms.YouTube.ScraperRefresh),
					)
					p.wg.Add(1)
					launchLoop(p.cfg, ytScraper)
				}
			}

			time.Sleep(1 * time.Second)
		}

		if p.value.FieldByName("Rumble").FieldByName("Priority").Int() == i || p.value.FieldByName("Rumble").FieldByName("Priority").Int() == 0 {
			if p.cfg.Platforms.Rumble.Enabled {
				if p.cfg.Platforms.Rumble.ScraperRefresh != 0 {
					r := rumble.New(p.cfg, p.state)
					slog.Info("running platform loop",
						r.GetPrefix(),
						slog.Int("refresh", p.cfg.Platforms.Rumble.ScraperRefresh),
					)
					p.wg.Add(1)
					launchLoop(p.cfg, r)
				}
			}

			time.Sleep(1 * time.Second)
		}

		if p.value.FieldByName("Kick").FieldByName("Priority").Int() == i || p.value.FieldByName("Kick").FieldByName("Priority").Int() == 0 {
			if p.cfg.Platforms.Kick.Enabled {
				if p.cfg.Platforms.Kick.ScraperRefresh != 0 {
					k := kick.New(p.cfg, p.state)
					slog.Info("running platform loop",
						k.GetPrefix(),
						slog.Int("refresh", p.cfg.Platforms.Kick.ScraperRefresh),
					)
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
	GetPrefix() slog.Attr
	GetSleepTime() time.Duration
}

func launchLoop(cfg *config.Config, imp implementation) {
	prefix := imp.GetPrefix()
	sleep := imp.GetSleepTime()

	go func() {
		l := lua.NewState()
		defer l.Close()
		if cfg.Plugins.Enabled {
			luaLibs.Preload(l)
			if err := l.DoFile(cfg.Plugins.PathToPlugin); err != nil {
				slog.Error("unable to load lua script", slog.Any("err", err))
				os.Exit(1)
			}
		}

		timeout := 0

		for {
			if timeout > 0 {
				slog.Info("sleeping before starting",
					prefix,
					slog.Int("duration", timeout),
				)
				time.Sleep(time.Second * time.Duration(timeout))
			}
			err := imp.CheckLivestream(l)
			if err != nil {
				slog.Error("error occurred while checking, restarting the loop",
					prefix,
					slog.Any("err", err),
				)
				switch {
				case timeout == 0:
					timeout = 1
				case (timeout >= 1 && timeout <= 32):
					timeout *= 2
				}
				continue
			}
			timeout = 0
			slog.Debug("sleeping",
				prefix,
				slog.Int("duration", int(sleep.Minutes())),
			)
			time.Sleep(sleep)
		}
	}()
}
