package rumble

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/DggHQ/dggarchiver-logger"
)

type oembed struct {
	Title     string `json:"title"`
	Duration  int    `json:"duration"`
	Thumbnail string `json:"thumbnail_url"`
	HTML      string `json:"html"`
	EmbedID   string
}

func getOEmbed(url string) *oembed {
	response, err := http.Get(fmt.Sprintf("https://rumble.com/api/Media/oembed.json/?url=%s", url))
	if err != nil {
		log.Errorf("[Rumble] [SCRAPER] HTTP error during the OEmbed check (%s): %s", url, err)
		return nil
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		log.Errorf("[Rumble] [SCRAPER] Status code != 200 for OEmbed, giving up.")
		return nil
	}

	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		log.Errorf("[Rumble] [SCRAPER] Read error during the OEmbed check (%s): %s", url, err)
		return nil
	}

	data := oembed{}
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		log.Errorf("[Rumble] [SCRAPER] Unmarshalling error during the OEmbed check (%s): %s", url, err)
		return nil
	}

	if data.HTML != "" {
		data.EmbedID = strings.SplitN(strings.SplitN(data.HTML, "https://rumble.com/embed/", 2)[1], "/", 2)[0]
	}

	return &data
}
