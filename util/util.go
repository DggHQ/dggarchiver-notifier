package util

import (
	"net/http"
	"time"

	log "github.com/DggHQ/dggarchiver-logger"
)

func HealthCheck(url *string) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	_, err := client.Head(*url)
	if err != nil {
		log.Errorf("HealthCheck error: %s", err)
	}
}
