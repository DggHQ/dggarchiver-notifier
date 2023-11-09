package util

import (
	"net/http"
	"time"

	log "github.com/DggHQ/dggarchiver-logger"
)

var healthCheckClient = &http.Client{
	Timeout: 10 * time.Second,
}

func HealthCheck(url string) {
	if url == "" {
		return
	}

	_, err := healthCheckClient.Head(url)
	if err != nil {
		log.Errorf("HealthCheck error: %s", err)
	}
}
