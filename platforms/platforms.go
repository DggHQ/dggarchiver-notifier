package platforms

import (
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	log "github.com/DggHQ/dggarchiver-logger"
	luaLibs "github.com/vadv/gopher-lua-libs"
	lua "github.com/yuin/gopher-lua"
)

type Implementation interface {
	CheckLivestream(*lua.LState) error
	GetPrefix() string
	GetSleepTime() time.Duration
}

func LaunchLoop(cfg *config.Config, imp Implementation) {
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
