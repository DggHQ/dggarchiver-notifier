package kick

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	config "github.com/DggHQ/dggarchiver-config"
	log "github.com/DggHQ/dggarchiver-logger"
	dggarchivermodel "github.com/DggHQ/dggarchiver-model"
	"github.com/DggHQ/dggarchiver-notifier/util"
	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/exp/slices"
)

var kickHTTPClient tls_client.HttpClient

func init() {
	var err error

	jar := tls_client.NewCookieJar()
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(tls_client.Chrome_110),
		tls_client.WithNotFollowRedirects(),
		tls_client.WithCookieJar(jar),
	}

	kickHTTPClient, err = tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		log.Fatalf("[Kick] [SCRAPER] Error while creating a TLS client: %s", err)
	}
}

func ScrapeKickStream(cfg *config.Config) *KickAPI {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://kick.com/api/v1/channels/%s", cfg.Notifier.Platforms.Kick.Channel), nil)
	if err != nil {
		log.Fatalf("[Kick] [SCRAPER] Error creating a request: %s", err)
	}

	req.Header = http.Header{
		"accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
		"accept-language": {"en-US,en;q=0.5"},
		"user-agent":      {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36"},
		http.HeaderOrderKey: {
			"accept",
			"accept-language",
			"user-agent",
		},
	}

	resp, err := kickHTTPClient.Do(req)
	if err != nil {
		log.Errorf("[Kick] [SCRAPER] Error making a request: %s", err)
		return nil
	}

	defer resp.Body.Close()
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("[Kick] [SCRAPER] Error reading the response: %s", err)
		return nil
	}
	var stream KickAPI
	err = json.Unmarshal(bytes, &stream)
	if err != nil {
		log.Errorf("[Kick] [SCRAPER] Error unmarshalling the response: %s", err)
		return nil
	}
	return &stream
}

func LoopScrapedLivestream(cfg *config.Config, state *util.State, L *lua.LState) error {
	stream := ScrapeKickStream(cfg)
	if stream != nil && stream.Livestream.IsLive {
		if !slices.Contains(state.SentVODs, fmt.Sprintf("kick:%d", stream.Livestream.ID)) {
			if state.CurrentStreams.YouTube.ID == "" {
				log.Infof("[Kick] [SCRAPER] Found a currently running stream with ID %d", stream.Livestream.ID)
				if cfg.Notifier.Plugins.Enabled {
					util.LuaCallReceiveFunction(L, fmt.Sprintf("%d", stream.Livestream.ID))
				}

				vod := &dggarchivermodel.VOD{
					Platform:    "kick",
					ID:          fmt.Sprintf("%d", stream.Livestream.ID),
					PlaybackURL: stream.URL,
					Title:       stream.Livestream.Title,
					StartTime:   time.Now().Format(time.RFC3339),
					EndTime:     "",
					Thumbnail:   strings.Split(strings.Split(stream.Livestream.Thumbnail.URL, ",")[0], " ")[0],
				}

				state.CurrentStreams.Kick = *vod

				bytes, err := json.Marshal(vod)
				if err != nil {
					log.Fatalf("[Kick] [SCRAPER] Couldn't marshal VOD with ID %s into a JSON object: %v", vod.ID, err)
				}

				if err = cfg.NATS.NatsConnection.Publish(fmt.Sprintf("%s.job", cfg.NATS.Topic), bytes); err != nil {
					log.Errorf("[Kick] [SCRAPER] Wasn't able to send message with VOD with ID %s: %v", vod.ID, err)
					return nil
				}

				if cfg.Notifier.Plugins.Enabled {
					util.LuaCallSendFunction(L, vod)
				}
				state.SentVODs = append(state.SentVODs, fmt.Sprintf("kick:%s", vod.ID))
				state.Dump()
			} else {
				log.Infof("[Kick] [SCRAPER] Stream with ID %d is being streamed on YouTube, skipping", stream.Livestream.ID)
			}
		} else {
			log.Infof("[Kick] [SCRAPER] Stream with ID %d was already sent", stream.Livestream.ID)
		}
	} else {
		state.CurrentStreams.Kick = dggarchivermodel.VOD{}
		log.Infof("[Kick] [SCRAPER] No stream found")
	}
	return nil
}
