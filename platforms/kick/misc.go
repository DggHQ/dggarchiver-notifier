package kick

import (
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	log "github.com/DggHQ/dggarchiver-logger"
	"github.com/DggHQ/dggarchiver-notifier/util"
	luaLibs "github.com/vadv/gopher-lua-libs"
	lua "github.com/yuin/gopher-lua"
)

type KickAPI struct {
	URL        string `json:"playback_url"`
	Livestream struct {
		IsLive    bool   `json:"is_live"`
		ID        int    `json:"id"`
		Slug      string `json:"slug"`
		CreatedAt string `json:"created_at"`
		Title     string `json:"session_title"`
		Thumbnail struct {
			URL string `json:"responsive"`
		} `json:"thumbnail"`
	} `json:"livestream"`
}

type loopKick func(*config.Config, *util.State, *lua.LState) error

func StartKickThread(prefix string, f loopKick, cfg *config.Config, state *util.State, sleeptime time.Duration) {
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
