package rumble

import (
	"strings"
	"time"

	config "github.com/DggHQ/dggarchiver-config"
	log "github.com/DggHQ/dggarchiver-logger"
	"github.com/DggHQ/dggarchiver-notifier/util"
	luaLibs "github.com/vadv/gopher-lua-libs"
	lua "github.com/yuin/gopher-lua"
)

type RumbleOembed struct {
	Title     string `json:"title"`
	Duration  int    `json:"duration"`
	Thumbnail string `json:"thumbnail_url"`
	HTML      string `json:"html"`
}

func (data RumbleOembed) EmbedID() string {
	if data.HTML == "" {
		return ""
	}
	return strings.SplitN(strings.SplitN(data.HTML, "https://rumble.com/embed/", 2)[1], "/", 2)[0]
}

type RumbleAPI struct {
	PubDate string `json:"pubDate"`
}

func (data RumbleAPI) StringToTime() *time.Time {
	if data.PubDate == "" {
		return nil
	}
	res, err := time.Parse(time.RFC3339, data.PubDate)
	if err != nil {
		return nil
	}
	return &res
}

type loopRumble func(*config.Config, *util.State, *lua.LState) error

func StartRumbleThread(prefix string, f loopRumble, cfg *config.Config, state *util.State, sleeptime time.Duration) {
	go func() {
		L := lua.NewState()
		defer L.Close()
		if cfg.Notifier.Plugins.Enabled {
			luaLibs.Preload(L)
			if err := L.DoFile(cfg.Notifier.Plugins.PathToPlugin); err != nil {
				log.Fatalf("Wasn't able to load the Lua script: %s", err)
			}
		}

		timeout := 0

		for {
			if timeout > 0 {
				log.Infof("%s Sleeping for %d seconds before starting...", prefix, timeout)
				time.Sleep(time.Second * time.Duration(timeout))
			}
			err := f(cfg, state, L)
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
			log.Infof("%s Sleeping for %.f minutes...", prefix, sleeptime.Minutes())
			time.Sleep(sleeptime)
		}
	}()
}
