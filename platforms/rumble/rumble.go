package rumble

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	config "github.com/DggHQ/dggarchiver-config"
	log "github.com/DggHQ/dggarchiver-logger"
	dggarchivermodel "github.com/DggHQ/dggarchiver-model"
	"github.com/DggHQ/dggarchiver-notifier/util"
	"github.com/gocolly/colly/v2"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/exp/slices"
)

func GetRumbleEmbedAPI(embedID string) *RumbleAPI {
	response, err := http.Get(fmt.Sprintf("https://rumble.com/embedJS/u3/?request=video&ver=2&v=%s&ext={\"ad_count\":null}&ad_wt=0", embedID))
	if err != nil {
		log.Errorf("[Rumble] [SCRAPER] HTTP error during the Rumble API check (%s): %s", embedID, err)
		return nil
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		log.Errorf("[Rumble] [SCRAPER] Status code != 200 for Rumble API, giving up.")
		return nil
	}

	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		log.Errorf("[Rumble] [SCRAPER] Read error during the Rumble API check (%s): %s", embedID, err)
		return nil
	}

	data := &RumbleAPI{}
	err = json.Unmarshal(bytes, data)
	if err != nil {
		log.Errorf("[Rumble] [SCRAPER] Unmarshalling error during the Rumble API check (%s): %s", embedID, err)
		return nil
	}

	return data
}

func GetRumbleEmbed(url string) *RumbleOembed {
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

	data := &RumbleOembed{}
	err = json.Unmarshal(bytes, data)
	if err != nil {
		log.Errorf("[Rumble] [SCRAPER] Unmarshalling error during the OEmbed check (%s): %s", url, err)
		return nil
	}

	return data
}

func ScrapeRumblePage(cfg *config.Config) *dggarchivermodel.VOD {
	var vod *dggarchivermodel.VOD
	c := colly.NewCollector()
	// disable cookie handling
	c.DisableCookies()

	c.OnHTML("li.video-listing-entry", func(h *colly.HTMLElement) {
		h.ForEach("a.video-item--a", func(_ int, h *colly.HTMLElement) {
			live := h.ChildAttr("span.video-item--live", "data-value")
			if len(live) != 0 {
				link := h.Attr("href")
				embedData := GetRumbleEmbed(link)
				embedID := embedData.EmbedID()
				vod = &dggarchivermodel.VOD{
					Platform:    "rumble",
					ID:          embedID,
					PlaybackURL: fmt.Sprintf("https://rumble.com%s", link),
					Title:       embedData.Title,
					StartTime:   time.Now().Format(time.RFC3339),
					EndTime:     "",
					Thumbnail:   embedData.Thumbnail,
				}
			}
		})
	})

	c.Visit(fmt.Sprintf("https://rumble.com/c/%s", cfg.Notifier.Platforms.Rumble.Channel))

	return vod
}

func LoopScrapedLivestream(cfg *config.Config, state *util.State, L *lua.LState) error {
	vod := ScrapeRumblePage(cfg)
	if vod != nil {
		if !slices.Contains(state.SentVODs, fmt.Sprintf("rumble:%s", vod.ID)) {
			if state.CheckPriority("Rumble", cfg) {
				log.Infof("[Rumble] [SCRAPER] Found a currently running stream with ID %s", vod.ID)
				if cfg.Notifier.Plugins.Enabled {
					util.LuaCallReceiveFunction(L, vod.ID)
				}

				state.CurrentStreams.Rumble = *vod

				bytes, err := json.Marshal(vod)
				if err != nil {
					log.Fatalf("[Rumble] [SCRAPER] Couldn't marshal VOD with ID %s into a JSON object: %v", vod.ID, err)
				}

				if err = cfg.NATS.NatsConnection.Publish(fmt.Sprintf("%s.job", cfg.NATS.Topic), bytes); err != nil {
					log.Errorf("[Rumble] [SCRAPER] Wasn't able to send message with VOD with ID %s: %v", vod.ID, err)
					return nil
				}

				if cfg.Notifier.Plugins.Enabled {
					util.LuaCallSendFunction(L, vod)
				}
				state.SentVODs = append(state.SentVODs, fmt.Sprintf("rumble:%s", vod.ID))
				state.Dump()
			} else {
				log.Infof("[Rumble] [SCRAPER] Stream with ID %s is being streamed on a different platform, skipping", vod.ID)
			}
		} else {
			log.Infof("[Rumble] [SCRAPER] Stream with ID %s was already sent", vod.ID)
		}
	} else {
		state.CurrentStreams.Rumble = dggarchivermodel.VOD{}
		log.Infof("[Rumble] [SCRAPER] No stream found")
	}
	return nil
}
