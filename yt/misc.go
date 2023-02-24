package yt

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	log "github.com/DggHQ/dggarchiver-logger"
	"github.com/DggHQ/dggarchiver-notifier/config"
	"github.com/DggHQ/dggarchiver-notifier/util"
	luaLibs "github.com/vadv/gopher-lua-libs"
	lua "github.com/yuin/gopher-lua"
)

type YTErrorWrapper struct {
	Message string
	Module  string
	Err     error
}

var ErrIsNotModified = errors.New("not modified")

var ytRegexp *regexp.Regexp = regexp.MustCompile(`\/watch\?v=([^\"]*)`)

type loopYT func(*config.Config, *util.State, *lua.LState) error

func StartYTThread(prefix string, f loopYT, cfg *config.Config, state *util.State, sleeptime time.Duration) {
	go func() {
		L := lua.NewState()
		defer L.Close()
		if cfg.PluginConfig.On {
			luaLibs.Preload(L)
			if err := L.DoFile(cfg.PluginConfig.PathToScript); err != nil {
				log.Fatalf("Wasn't able to load the Lua script: %s", err)
			}
		} else {
			L.Close()
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

func (err *YTErrorWrapper) Error() string {
	if err.Module == "" {
		return fmt.Sprintf("[YT] %s: %v", err.Message, err.Err)
	} else {
		return fmt.Sprintf("[YT] [%s] %s: %v", err.Module, err.Message, err.Err)
	}
}

func (err *YTErrorWrapper) Unwrap() error {
	return err.Err
}

func WrapWithYTError(err error, module string, message string) error {
	return &YTErrorWrapper{
		Message: message,
		Module:  module,
		Err:     err,
	}
}
