package util

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	log "github.com/DggHQ/dggarchiver-notifier/logger"
)

type State struct {
	SearchETag string
	SentVODs   []string
}

func (state *State) Dump() {
	file, _ := json.MarshalIndent(state, "", "	")
	err := os.WriteFile("./data/state.json", file, 0644)
	if err != nil {
		log.Fatalf("State dump error: %s", err)
	}
}

func (state *State) Load() {
	bytes, err := os.ReadFile("./data/state.json")
	if err != nil {
		log.Errorf("State load error, skipping load: %s", err)
		return
	}
	err = json.Unmarshal(bytes, state)
	if err != nil {
		log.Fatalf("State unmarshal error: %s", err)
	}
}

func HealthCheck(url *string) {
	var client = &http.Client{
		Timeout: 10 * time.Second,
	}

	_, err := client.Head(*url)
	if err != nil {
		log.Errorf("HealthCheck error: %s", err)
	}
}
