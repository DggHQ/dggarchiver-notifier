package yt

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	log "github.com/DggHQ/dggarchiver-logger"
	"github.com/DggHQ/dggarchiver-notifier/util"
	luaLibs "github.com/vadv/gopher-lua-libs"
	lua "github.com/yuin/gopher-lua"
)

type ErrorWrapper struct {
	Message string
	Module  string
	Err     error
}

var ErrIsNotModified = errors.New("not modified")

var ytRegexp = regexp.MustCompile(`\/watch\?v=([^\"]*)`)

type loopYT func(*config.Config, *util.State, *lua.LState) error

func StartYTThread(prefix string, f loopYT, cfg *config.Config, state *util.State, sleeptime time.Duration) {
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

func (err *ErrorWrapper) Error() string {
	if err.Module == "" {
		return fmt.Sprintf("[YT] %s: %v", err.Message, err.Err)
	}

	return fmt.Sprintf("[YT] [%s] %s: %v", err.Module, err.Message, err.Err)
}

func (err *ErrorWrapper) Unwrap() error {
	return err.Err
}

func WrapWithYTError(err error, module string, message string) error {
	return &ErrorWrapper{
		Message: message,
		Module:  module,
		Err:     err,
	}
}
