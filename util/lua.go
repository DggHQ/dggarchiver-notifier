package util

import (
	dggarchivermodel "github.com/DggHQ/dggarchiver-model"
	"github.com/apex/log"
	lua "github.com/yuin/gopher-lua"
	luar "layeh.com/gopher-luar"
)

type LuaResponse struct {
	Filled  bool
	Error   bool
	Message string
	Data    map[string]interface{}
}

func LuaCallReceiveFunction(l *lua.LState, vod string) *LuaResponse {
	luaVOD := luar.New(l, vod)

	result := &LuaResponse{}
	l.SetGlobal("ReceiveResponse", luar.New(l, result))

	if err := l.CallByParam(lua.P{
		Fn:      l.GetGlobal("OnReceive"),
		NRet:    0,
		Protect: true,
	}, luaVOD); err != nil {
		log.Debugf("Wasn't able to access the \"OnReceive\" function of the Lua script, skipping: %s", err)
		return nil
	}

	if result.Filled {
		if result.Error {
			log.Errorf("Wasn't able to execute the \"OnReceive\" function of the Lua script: %s", result.Message)
			return nil
		}
	}

	return result
}

func LuaCallSendFunction(l *lua.LState, vod *dggarchivermodel.VOD) *LuaResponse {
	luaVOD := luar.New(l, vod)

	result := &LuaResponse{}
	l.SetGlobal("SendResponse", luar.New(l, result))

	if err := l.CallByParam(lua.P{
		Fn:      l.GetGlobal("OnSend"),
		NRet:    0,
		Protect: true,
	}, luaVOD); err != nil {
		log.Debugf("Wasn't able to access the \"OnSend\" function of the Lua script, skipping: %s", err)
		return nil
	}

	if result.Filled {
		if result.Error {
			log.Errorf("Wasn't able to execute the \"OnSend\" function of the Lua script: %s", result.Message)
			return nil
		}
	}

	return result
}
