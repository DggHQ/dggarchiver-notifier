package util

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	log "github.com/DggHQ/dggarchiver-logger"
	dggarchivermodel "github.com/DggHQ/dggarchiver-model"
)

type State struct {
	SearchETag     string
	SentVODs       []string
	CurrentStreams struct {
		YouTube dggarchivermodel.VOD
		Rumble  dggarchivermodel.VOD
		Kick    dggarchivermodel.VOD
	} `json:"-"`
}

func (state *State) CheckPriority(platformName string, config *config.Config) bool {
	stateValue := reflect.ValueOf(state.CurrentStreams)
	platformsValue := reflect.ValueOf(config.Notifier.Platforms)
	priority := platformsValue.FieldByName(platformName).FieldByName("Priority").Int()
	if priority <= 1 {
		return true
	}
	platformsFields := reflect.VisibleFields(reflect.TypeOf(config.Notifier.Platforms))
	for _, field := range platformsFields {
		if field.Name != platformName {
			if platformsValue.FieldByName(field.Name).FieldByName("Priority").Int() < priority && stateValue.FieldByName(field.Name).FieldByName("ID").String() != "" {
				return false
			}
		}
	}
	return false
}

func (state *State) Dump() {
	file, _ := json.MarshalIndent(state, "", "	")
	err := os.WriteFile("./data/state.json", file, 0o644)
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
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	_, err := client.Head(*url)
	if err != nil {
		log.Errorf("HealthCheck error: %s", err)
	}
}
